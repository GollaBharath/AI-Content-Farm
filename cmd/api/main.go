package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/config"
	"github.com/Gollabharath/ai-content-farm/internal/httpserver"
	"github.com/Gollabharath/ai-content-farm/internal/pipeline"
	"github.com/Gollabharath/ai-content-farm/internal/storage"
	"github.com/Gollabharath/ai-content-farm/internal/trends"
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

	store := storage.NewJobStore()
	runner := pipeline.NewRunner(
		store,
		trends.NewStaticProvider(),
		tts.NewHTTPClient(cfg.TTSBaseURL, cfg.TTSSynthPath, time.Duration(cfg.TTSTimeoutSecs)*time.Second),
		video.NewFFmpegBuilder(cfg.FFmpegBinaryPath),
		cfg.StorageDir,
		2,
	)
	runner.Start(ctx)
	defer runner.Stop()

	srv := httpserver.New(store, runner)
	addr := ":" + cfg.Port
	log.Printf("api listening on %s", addr)

	if err := httpserver.ListenAndServe(ctx, addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
