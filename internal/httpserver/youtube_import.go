package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	youtubeClipDurationSeconds       = 56
	youtubeTailDeleteThresholdSecond = 55.5
	youtubeMaxClipBytes              = 35 * 1024 * 1024
)

var youtubeURLRe = regexp.MustCompile(`^(https?://)?(www\.)?(youtube\.com|youtu\.be)/`)

func (s *Server) handleImportYouTube(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	url := strings.TrimSpace(req.URL)
	if !youtubeURLRe.MatchString(url) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid YouTube URL"})
		return
	}

	cfg, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.MkdirAll(cfg.InputVideosDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	importsRoot := filepath.Join(cfg.InputVideosDir, "uploads")
	if err := os.MkdirAll(importsRoot, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	workingDir, err := os.MkdirTemp("", "youtube-import-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer os.RemoveAll(workingDir)

	sourcePath, err := downloadYouTubeVideo(r.Context(), url, workingDir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	batchFolderName := fmt.Sprintf("youtube-%d", time.Now().Unix())
	destDir := filepath.Join(importsRoot, batchFolderName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	clips, err := splitYouTubeVideoIntoClips(r.Context(), sourcePath, destDir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	compressedCount := 0
	for _, c := range clips {
		if c.Compressed {
			compressedCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source_url":       url,
		"clips_created":    len(clips),
		"clips_compressed": compressedCount,
		"clips":            clips,
		"upload_dir":       filepath.ToSlash(filepath.Join("uploads", batchFolderName)),
	})
}

func downloadYouTubeVideo(ctx context.Context, sourceURL, workingDir string) (string, error) {
	outTemplate := filepath.Join(workingDir, "source.%(ext)s")
	attempts := [][]string{
		{
			"--no-playlist",
			"--extractor-args", "youtube:player_client=android,web,mweb",
			"--merge-output-format", "mp4",
			"-f", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/bv*+ba/b",
			"-o", outTemplate,
			sourceURL,
		},
		{
			"--no-playlist",
			"--extractor-args", "youtube:player_client=web,mweb",
			"--merge-output-format", "mp4",
			"-f", "b/bv*+ba",
			"-o", outTemplate,
			sourceURL,
		},
		{
			"--no-playlist",
			"--extractor-args", "youtube:player_client=android",
			"--merge-output-format", "mp4",
			"-f", "best",
			"-o", outTemplate,
			sourceURL,
		},
	}

	var lastErr string
	for _, args := range attempts {
		cmd := exec.CommandContext(ctx, "yt-dlp", args...)
		raw, err := cmd.CombinedOutput()
		if err == nil {
			lastErr = ""
			break
		}
		lastErr = strings.TrimSpace(string(raw))
	}
	if lastErr != "" {
		return "", fmt.Errorf("yt-dlp failed: %s", lastErr)
	}

	entries, err := os.ReadDir(workingDir)
	if err != nil {
		return "", fmt.Errorf("read download directory: %w", err)
	}

	candidates := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(strings.ToLower(name), "source.") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".mp4", ".mov", ".mkv", ".webm":
			candidates = append(candidates, filepath.Join(workingDir, name))
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("download finished but no video file was created")
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

type importedClip struct {
	Filename   string  `json:"filename"`
	Duration   float64 `json:"duration"`
	Size       int64   `json:"size"`
	Compressed bool    `json:"compressed"`
}

func splitYouTubeVideoIntoClips(ctx context.Context, sourcePath, destDir string) ([]importedClip, error) {
	ext := ".mp4"
	safeBase := fmt.Sprintf("%s_%d", sanitizeYouTubeBase(filepath.Base(sourcePath)), time.Now().UnixNano())
	segmentPattern := filepath.Join(destDir, safeBase+"_part_%03d"+ext)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", sourcePath,
		"-map", "0",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", youtubeClipDurationSeconds),
		"-f", "segment",
		"-segment_time", strconv.Itoa(youtubeClipDurationSeconds),
		"-reset_timestamps", "1",
		segmentPattern,
	)
	if raw, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg split failed: %s", strings.TrimSpace(string(raw)))
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return nil, fmt.Errorf("read destination directory: %w", err)
	}

	created := make([]string, 0, 16)
	prefix := safeBase + "_part_"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.ToLower(filepath.Ext(name)) != ext {
			continue
		}
		created = append(created, filepath.Join(destDir, name))
	}
	if len(created) == 0 {
		return nil, fmt.Errorf("video split finished but no clips were created")
	}
	sort.Strings(created)

	kept := make([]importedClip, 0, len(created))
	for _, clipPath := range created {
		duration, err := ffprobeDuration(ctx, clipPath)
		if err != nil || duration < youtubeTailDeleteThresholdSecond {
			_ = os.Remove(clipPath)
			continue
		}
		st, err := os.Stat(clipPath)
		if err != nil {
			_ = os.Remove(clipPath)
			continue
		}

		compressed := false
		size := st.Size()
		if size > youtubeMaxClipBytes {
			newSize, compErr := compressClipInPlace(ctx, clipPath)
			if compErr == nil {
				compressed = true
				size = newSize
			}
		}

		kept = append(kept, importedClip{
			Filename:   filepath.Base(clipPath),
			Duration:   duration,
			Size:       size,
			Compressed: compressed,
		})
	}

	return kept, nil
}

func compressClipInPlace(ctx context.Context, clipPath string) (int64, error) {
	tmpPath := clipPath + ".compressed.mp4"
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", clipPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "21",
		"-maxrate", "4M",
		"-bufsize", "8M",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		tmpPath,
	)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("ffmpeg compress failed: %s", strings.TrimSpace(string(raw)))
	}
	if err := os.Rename(tmpPath, clipPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("replace compressed clip: %w", err)
	}
	st, err := os.Stat(clipPath)
	if err != nil {
		return 0, fmt.Errorf("stat compressed clip: %w", err)
	}
	return st.Size(), nil
}

func ffprobeDuration(ctx context.Context, filePath string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %s", strings.TrimSpace(string(raw)))
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return d, nil
}

func sanitizeYouTubeBase(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.ToLower(strings.TrimSpace(base))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	base = re.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return "youtube"
	}
	return base
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
