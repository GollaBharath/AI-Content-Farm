package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
	"github.com/Gollabharath/ai-content-farm/internal/pipeline"
	"github.com/Gollabharath/ai-content-farm/internal/settings"
	"github.com/Gollabharath/ai-content-farm/internal/storage"
	"github.com/Gollabharath/ai-content-farm/internal/tts"
)

type Server struct {
	store             *storage.JobStore
	settings          *settings.Store
	runner            *pipeline.Runner
	tts               tts.Client
	onSettingsUpdated func(context.Context, settings.Settings, settings.Settings)
	mux               *http.ServeMux
}

func New(
	store *storage.JobStore,
	settingsStore *settings.Store,
	runner *pipeline.Runner,
	ttsClient tts.Client,
	onSettingsUpdated func(context.Context, settings.Settings, settings.Settings),
) *Server {
	s := &Server{
		store:             store,
		settings:          settingsStore,
		runner:            runner,
		tts:               ttsClient,
		onSettingsUpdated: onSettingsUpdated,
		mux:               http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	webRoot, _ := fs.Sub(uiFS, "web")
	s.mux.Handle("GET /app.js", http.FileServerFS(webRoot))
	s.mux.Handle("GET /styles.css", http.FileServerFS(webRoot))
	s.mux.Handle("GET /", http.FileServerFS(webRoot))
	s.mux.HandleFunc("GET /outputs/", s.handleOutputFile)
	s.mux.HandleFunc("GET /inputs/", s.handleInputFile)

	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /v1/scripts/generate", s.handleGenerateScript)
	s.mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	s.mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("DELETE /v1/jobs", s.handleClearJobs)
	s.mux.HandleFunc("POST /v1/jobs/", s.handleRerunJob)
	s.mux.HandleFunc("GET /v1/jobs/", s.handleGetJob)

	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)
	s.mux.HandleFunc("GET /api/voices", s.handleListVoices)
	s.mux.HandleFunc("POST /api/voices/preview", s.handlePreviewVoice)
	s.mux.HandleFunc("GET /api/videos", s.handleListVideos)
	s.mux.HandleFunc("GET /api/videos/generated", s.handleListGeneratedVideos)
	s.mux.HandleFunc("POST /api/videos/import-youtube", s.handleImportYouTube)
	s.mux.HandleFunc("POST /api/videos/upload", s.handleUploadVideos)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGenerateScript(w http.ResponseWriter, r *http.Request) {
	var req job.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	generated, err := s.runner.GenerateScript(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, generated)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req job.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	if strings.TrimSpace(req.Prompt) == "" && strings.TrimSpace(req.ScriptOverride) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing content: provide prompt or script_override"})
		return
	}

	j, err := s.runner.CreateJob(req)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleListJobs(w http.ResponseWriter, _ *http.Request) {
	jobs := s.store.List()
	for i := range jobs {
		jobs[i].OutputPath = s.publicOutputPath(jobs[i].OutputPath)
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleClearJobs(w http.ResponseWriter, _ *http.Request) {
	if err := s.store.Clear(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing job id"})
		return
	}
	job, ok := s.store.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	job.OutputPath = s.publicOutputPath(job.OutputPath)
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleRerunJob(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if !strings.HasSuffix(path, "/rerun") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unsupported job action"})
		return
	}

	id := strings.TrimSuffix(path, "/rerun")
	id = strings.TrimSuffix(id, "/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing job id"})
		return
	}

	oldJob, ok := s.store.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	if oldJob.Status != job.StatusFailed {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only failed jobs can be re-run"})
		return
	}

	newJob, err := s.runner.CreateJob(oldJob.Request)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, newJob)
}

func (s *Server) publicOutputPath(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	name := filepath.Base(raw)
	if name == "." || name == "/" || name == "" {
		return ""
	}
	return "/outputs/" + name
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	before, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var update settings.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	cfg, err := s.settings.Update(update)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if s.onSettingsUpdated != nil {
		s.onSettingsUpdated(r.Context(), before, cfg)
	}

	_ = os.MkdirAll(cfg.InputVideosDir, 0o755)
	_ = os.MkdirAll(cfg.OutputVideosDir, 0o755)
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleListVoices(w http.ResponseWriter, r *http.Request) {
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))

	type providerVoiceLister interface {
		ListVoicesForProvider(context.Context, string) ([]tts.Voice, error)
	}
	type providerLanguageLister interface {
		ListSupportedLanguagesForProvider(context.Context, string) ([]string, error)
	}

	var (
		voices []tts.Voice
		err    error
	)
	if provider != "" {
		if lister, ok := s.tts.(providerVoiceLister); ok {
			voices, err = lister.ListVoicesForProvider(r.Context(), provider)
		} else {
			voices, err = s.tts.ListVoices(r.Context())
		}
	} else {
		voices, err = s.tts.ListVoices(r.Context())
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	requestedLanguage := strings.TrimSpace(r.URL.Query().Get("language"))
	if requestedLanguage != "" {
		filtered := make([]tts.Voice, 0, len(voices))
		for _, v := range voices {
			if strings.EqualFold(v.LanguageCode, requestedLanguage) {
				filtered = append(filtered, v)
			}
		}
		voices = filtered
	}

	langsMap := map[string]struct{}{}
	for _, v := range voices {
		if strings.TrimSpace(v.LanguageCode) == "" {
			continue
		}
		langsMap[v.LanguageCode] = struct{}{}
	}
	languages := make([]string, 0, len(langsMap))
	for lang := range langsMap {
		languages = append(languages, lang)
	}
	if provider != "" {
		if lister, ok := s.tts.(providerLanguageLister); ok {
			extra, langErr := lister.ListSupportedLanguagesForProvider(r.Context(), provider)
			if langErr == nil {
				for _, lang := range extra {
					trimmed := strings.TrimSpace(lang)
					if trimmed == "" {
						continue
					}
					langsMap[trimmed] = struct{}{}
				}
			}
		}
		languages = languages[:0]
		for lang := range langsMap {
			languages = append(languages, lang)
		}
	}
	sort.Strings(languages)

	writeJSON(w, http.StatusOK, map[string]any{
		"voices":    voices,
		"languages": languages,
	})
}

func (s *Server) handlePreviewVoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text     string `json:"text"`
		Voice    string `json:"voice"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	audio, err := s.tts.Preview(r.Context(), req.Text, req.Voice, req.Language)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

func (s *Server) handleListVideos(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = os.MkdirAll(cfg.InputVideosDir, 0o755)

	entries, err := os.ReadDir(cfg.InputVideosDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	videos := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp4", ".mov", ".mkv", ".webm":
			videos = append(videos, map[string]string{
				"name": entry.Name(),
				"url":  "/inputs/" + entry.Name(),
			})
		}
	}

	sort.Slice(videos, func(i, j int) bool { return videos[i]["name"] < videos[j]["name"] })
	writeJSON(w, http.StatusOK, videos)
}

func (s *Server) handleListGeneratedVideos(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = os.MkdirAll(cfg.OutputVideosDir, 0o755)

	entries, err := os.ReadDir(cfg.OutputVideosDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	videos := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".mp4", ".mov", ".mkv", ".webm":
			videos = append(videos, map[string]string{
				"name": entry.Name(),
				"url":  "/outputs/" + entry.Name(),
			})
		}
	}

	sort.Slice(videos, func(i, j int) bool { return videos[i]["name"] < videos[j]["name"] })
	writeJSON(w, http.StatusOK, videos)
}

func (s *Server) handleUploadVideos(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(cfg.InputVideosDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart payload"})
		return
	}

	files := r.MultipartForm.File["videos"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no files in form field 'videos'"})
		return
	}

	uploaded := make([]string, 0, len(files))
	for _, header := range files {
		name := filepath.Base(header.Filename)
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".mp4", ".mov", ".mkv", ".webm":
		default:
			continue
		}

		src, err := header.Open()
		if err != nil {
			continue
		}
		dstPath := filepath.Join(cfg.InputVideosDir, name)
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			continue
		}
		_, _ = io.Copy(dst, src)
		_ = dst.Close()
		_ = src.Close()
		uploaded = append(uploaded, name)
	}

	writeJSON(w, http.StatusOK, map[string]any{"uploaded": uploaded})
}

func (s *Server) handleOutputFile(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveNamedFile(w, r, "/outputs/", cfg.OutputVideosDir)
}

func (s *Server) handleInputFile(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.settings.Get()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveNamedFile(w, r, "/inputs/", cfg.InputVideosDir)
}

func serveNamedFile(w http.ResponseWriter, r *http.Request, prefix, root string) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, prefix))
	if name == "" || name == "." || name == "/" {
		http.NotFound(w, r)
		return
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, name))
}

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
