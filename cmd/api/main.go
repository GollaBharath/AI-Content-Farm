package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/autopilot"
	"github.com/Gollabharath/ai-content-farm/internal/config"
	"github.com/Gollabharath/ai-content-farm/internal/httpserver"
	"github.com/Gollabharath/ai-content-farm/internal/pipeline"
	"github.com/Gollabharath/ai-content-farm/internal/script"
	"github.com/Gollabharath/ai-content-farm/internal/settings"
	"github.com/Gollabharath/ai-content-farm/internal/storage"
	"github.com/Gollabharath/ai-content-farm/internal/tts"
	"github.com/Gollabharath/ai-content-farm/internal/video"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.StorageDir, 0o755); err != nil {
		log.Fatalf("storage dir init error: %v", err)
	}

	dbPath := cfg.DatabasePath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(cfg.StorageDir, dbPath)
	}

	settingsStore, err := settings.NewStore(dbPath, settings.Settings{
		InputVideosDir:          cfg.InputVideosDir,
		OutputVideosDir:         cfg.OutputVideosDir,
		DefaultVideoOrientation: "portrait",
		DefaultVideoWidth:       1080,
		DefaultVideoHeight:      1920,
		TTSProvider:             cfg.TTSProvider,
		DefaultVoice:            "",
		DefaultLanguage:         "",
		DefaultPromptIdea:       "",
		PromptPresets:           []settings.Preset{},
	})
	if err != nil {
		log.Fatalf("settings init error: %v", err)
	}

	store, err := storage.NewJobStoreWithFile(dbPath)
	if err != nil {
		log.Fatalf("storage init error: %v", err)
	}
	var composeManager *tts.ComposeManager
	if cfg.TTSDockerAutoManage {
		composeManager = tts.NewComposeManager(
			tts.ResolveProjectDir(cfg.TTSDockerProjectDir),
			cfg.TTSDockerServiceName,
		)
	}

	syncTTSDocker := func(_ context.Context, provider string) {
		if composeManager == nil {
			return
		}
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := composeManager.SyncForProvider(syncCtx, provider); err != nil {
			log.Printf("tts docker sync failed for provider %q: %v", provider, err)
		}
	}

	providerFromSettings := func() string {
		current, err := settingsStore.Get()
		if err != nil {
			return cfg.TTSProvider
		}
		return current.TTSProvider
	}

	syncTTSDocker(ctx, providerFromSettings())

	piperClient := tts.NewHTTPClient(cfg.TTSBaseURL, cfg.TTSSynthPath, time.Duration(cfg.TTSTimeoutSecs)*time.Second)
	elevenLabsClient := tts.NewElevenLabsClient(tts.ElevenLabsConfig{
		APIKey:       cfg.ElevenLabsAPIKey,
		BaseURL:      cfg.ElevenLabsBaseURL,
		DefaultVoice: cfg.ElevenLabsVoiceID,
		DefaultModel: cfg.ElevenLabsModelID,
		OutputFormat: cfg.ElevenLabsOutputFmt,
		Timeout:      time.Duration(cfg.TTSTimeoutSecs) * time.Second,
	})
	ttsClient := tts.NewProviderRouter(
		piperClient,
		elevenLabsClient,
		func(context.Context) string {
			return providerFromSettings()
		},
		func(ctx context.Context) {
			provider := tts.ProviderPiper
			if _, err := settingsStore.Update(settings.Update{TTSProvider: &provider}); err != nil {
				log.Printf("auto fallback to piper failed to persist: %v", err)
				return
			}
			syncTTSDocker(ctx, provider)
		},
	)
	runner := pipeline.NewRunner(
		store,
		settingsStore,
		script.NewGeminiOpenRouterGenerator(cfg.GeminiAPIKey, cfg.OpenRouterAPIKey, cfg.OpenRouterModel, 60*time.Second),
		ttsClient,
		video.NewFFmpegBuilder(cfg.FFmpegBinaryPath),
		2,
	)
	runner.Start(ctx)
	defer runner.Stop()

	if cfg.AutoPilotEnabled {
		go autopilot.Start(ctx, runner, autopilot.Config{
			EverySeconds: cfg.AutoPilotEvery,
			Topic:        cfg.AutoTopic,
			Prompt:       cfg.AutoPrompt,
			Voice:        cfg.AutoVoice,
		})
		log.Printf("autopilot enabled: every %ds", cfg.AutoPilotEvery)
	}

	srv := httpserver.New(store, settingsStore, runner, ttsClient, func(ctx context.Context, before, after settings.Settings) {
		if strings.EqualFold(before.TTSProvider, after.TTSProvider) {
			return
		}
		syncTTSDocker(ctx, after.TTSProvider)
	})
	addr := ":" + cfg.Port
	log.Printf("api listening on %s", addr)

	if err := httpserver.ListenAndServe(ctx, addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
