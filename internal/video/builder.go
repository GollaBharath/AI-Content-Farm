package video

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Builder interface {
	Render(ctx context.Context, audioPath, topic, outDir string) (string, error)
}

type FFmpegBuilder struct {
	bin string
}

func NewFFmpegBuilder(bin string) *FFmpegBuilder {
	if strings.TrimSpace(bin) == "" {
		bin = "ffmpeg"
	}
	return &FFmpegBuilder{bin: bin}
}

func (b *FFmpegBuilder) Render(ctx context.Context, audioPath, topic, outDir string) (string, error) {
	if audioPath == "" {
		return "", fmt.Errorf("audio path cannot be empty")
	}
	if _, err := os.Stat(audioPath); err != nil {
		return "", fmt.Errorf("audio file unavailable: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	if topic == "" {
		topic = "untitled"
	}
	slug := sanitize(topic)
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%d.mp4", slug, time.Now().UnixNano()))

	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", "color=c=black:s=1080x1920:r=30",
		"-i", audioPath,
		"-shortest",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-movflags", "+faststart",
		outPath,
	}

	cmd := exec.CommandContext(ctx, b.bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return outPath, nil
}

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	if s == "" {
		return "untitled"
	}
	return s
}
