package autopilot

import (
	"context"
	"log"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
	"github.com/Gollabharath/ai-content-farm/internal/pipeline"
)

type Config struct {
	EverySeconds int
	Prompt       string
	Voice        string
}

func Start(ctx context.Context, runner *pipeline.Runner, cfg Config) {
	if cfg.EverySeconds < 30 {
		cfg.EverySeconds = 1800
	}

	enqueue := func() {
		req := job.Request{
			Prompt: cfg.Prompt,
			Voice:  cfg.Voice,
		}
		j, err := runner.CreateJob(req)
		if err != nil {
			log.Printf("autopilot: create job failed: %v", err)
			return
		}
		log.Printf("autopilot: queued job %s", j.ID)
	}

	// Warm-start generation once at boot so the farm starts producing immediately.
	enqueue()

	ticker := time.NewTicker(time.Duration(cfg.EverySeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			enqueue()
		}
	}
}
