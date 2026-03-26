package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
	"github.com/Gollabharath/ai-content-farm/internal/storage"
	"github.com/Gollabharath/ai-content-farm/internal/trends"
	"github.com/Gollabharath/ai-content-farm/internal/tts"
	"github.com/Gollabharath/ai-content-farm/internal/video"
)

type Runner struct {
	jobs    chan string
	store   *storage.JobStore
	trends  trends.Provider
	tts     tts.Client
	video   video.Builder
	outDir  string
	workers int
	wg      sync.WaitGroup
}

func NewRunner(store *storage.JobStore, trendsProvider trends.Provider, ttsClient tts.Client, videoBuilder video.Builder, outDir string, workers int) *Runner {
	if workers < 1 {
		workers = 1
	}
	return &Runner{
		jobs:    make(chan string, 100),
		store:   store,
		trends:  trendsProvider,
		tts:     ttsClient,
		video:   videoBuilder,
		outDir:  outDir,
		workers: workers,
	}
}

func (r *Runner) Start(ctx context.Context) {
	for i := 0; i < r.workers; i++ {
		r.wg.Add(1)
		go func(workerID int) {
			defer r.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case id, ok := <-r.jobs:
					if !ok {
						return
					}
					r.process(ctx, workerID, id)
				}
			}
		}(i + 1)
	}
}

func (r *Runner) Stop() {
	close(r.jobs)
	r.wg.Wait()
}

func (r *Runner) Enqueue(jobID string) error {
	select {
	case r.jobs <- jobID:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

func (r *Runner) process(ctx context.Context, workerID int, jobID string) {
	j, ok := r.store.Get(jobID)
	if !ok {
		log.Printf("worker %d: job not found: %s", workerID, jobID)
		return
	}

	j.Status = job.StatusRunning
	j.UpdatedAt = time.Now().UTC()
	r.store.Save(j)

	if err := os.MkdirAll(r.outDir, 0o755); err != nil {
		r.failJob(j, err)
		return
	}

	topic := strings.TrimSpace(j.Request.Topic)
	if topic == "" {
		topics, err := r.trends.TopTopics(ctx, j.Request.Category, j.Request.CountryCode, 1)
		if err != nil || len(topics) == 0 {
			r.failJob(j, fmt.Errorf("unable to derive topic: %w", err))
			return
		}
		topic = topics[0]
	}

	voiceoverPath, err := r.tts.Synthesize(ctx, "placeholder script for "+topic, j.Request.Voice, r.outDir)
	if err != nil {
		r.failJob(j, err)
		return
	}

	slug := sanitize(topic)
	videoPath, err := r.video.Render(ctx, voiceoverPath, slug, r.outDir)
	if err != nil {
		r.failJob(j, err)
		return
	}

	j.Status = job.StatusCompleted
	j.OutputPath = videoPath
	j.UpdatedAt = time.Now().UTC()
	r.store.Save(j)
}

func (r *Runner) failJob(j job.Job, err error) {
	j.Status = job.StatusFailed
	j.ErrorMessage = err.Error()
	j.UpdatedAt = time.Now().UTC()
	r.store.Save(j)
}

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	if s == "" {
		return "untitled"
	}
	return filepath.Clean(s)
}
