package httpserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
	"github.com/Gollabharath/ai-content-farm/internal/pipeline"
	"github.com/Gollabharath/ai-content-farm/internal/storage"
)

type Server struct {
	store  *storage.JobStore
	runner *pipeline.Runner
	mux    *http.ServeMux
}

func New(store *storage.JobStore, runner *pipeline.Runner) *Server {
	s := &Server{
		store:  store,
		runner: runner,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	s.mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("GET /v1/jobs/", s.handleGetJob)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req job.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	id := newJobID()
	now := time.Now().UTC()
	j := job.Job{
		ID:        id,
		Status:    job.StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Request:   req,
	}

	s.store.Save(j)
	if err := s.runner.Enqueue(id); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleListJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
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
	writeJSON(w, http.StatusOK, job)
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
