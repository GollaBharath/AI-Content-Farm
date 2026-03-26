package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
	ttsClient := tts.NewHTTPClient(cfg.TTSBaseURL, cfg.TTSSynthPath, time.Duration(cfg.TTSTimeoutSecs)*time.Second)
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

	srv := httpserver.New(store, settingsStore, runner, ttsClient)
	addr := ":" + cfg.Port
	log.Printf("api listening on %s", addr)

	if err := httpserver.ListenAndServe(ctx, addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
