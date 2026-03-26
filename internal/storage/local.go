package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
	_ "modernc.org/sqlite"
)

type JobStore struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewJobStoreWithFile(filePath string) (*JobStore, error) {
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &JobStore{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *JobStore) Save(j job.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reqRaw, _ := json.Marshal(j.Request)
	_, _ = s.db.Exec(`
		INSERT INTO jobs(id, status, created_at, updated_at, request_json, script, output_path, error_message)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			updated_at=excluded.updated_at,
			request_json=excluded.request_json,
			script=excluded.script,
			output_path=excluded.output_path,
			error_message=excluded.error_message
	`, j.ID, string(j.Status), j.CreatedAt.UTC().Format(timeLayout), j.UpdatedAt.UTC().Format(timeLayout), string(reqRaw), j.Script, j.OutputPath, j.ErrorMessage)
}

func (s *JobStore) Get(id string) (job.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRow(`
		SELECT id, status, created_at, updated_at, request_json, script, output_path, error_message
		FROM jobs WHERE id = ?
	`, id)
	j, err := scanJob(row)
	if err != nil {
		return job.Job{}, false
	}
	return j, true
}

func (s *JobStore) List() []job.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
		SELECT id, status, created_at, updated_at, request_json, script, output_path, error_message
		FROM jobs
	`)
	if err != nil {
		return []job.Job{}
	}
	defer rows.Close()

	result := make([]job.Job, 0, 32)
	for rows.Next() {
		j, err := scanJob(rows)
		if err == nil {
			result = append(result, j)
		}
	}
	sort.Slice(result, func(i, k int) bool {
		return result[i].CreatedAt.After(result[k].CreatedAt)
	})
	return result
}

func (s *JobStore) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			request_json TEXT NOT NULL,
			script TEXT,
			output_path TEXT,
			error_message TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("create jobs table: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func scanJob(s scanner) (job.Job, error) {
	var (
		id         string
		status     string
		createdAt  string
		updatedAt  string
		requestRaw string
		scriptText sql.NullString
		outputPath sql.NullString
		errMsg     sql.NullString
	)
	if err := s.Scan(&id, &status, &createdAt, &updatedAt, &requestRaw, &scriptText, &outputPath, &errMsg); err != nil {
		return job.Job{}, err
	}

	created, err := sqlTime(createdAt)
	if err != nil {
		return job.Job{}, err
	}
	updated, err := sqlTime(updatedAt)
	if err != nil {
		return job.Job{}, err
	}

	var req job.Request
	if err := json.Unmarshal([]byte(requestRaw), &req); err != nil {
		return job.Job{}, err
	}

	return job.Job{
		ID:           id,
		Status:       job.Status(status),
		CreatedAt:    created,
		UpdatedAt:    updated,
		Request:      req,
		Script:       scriptText.String,
		OutputPath:   outputPath.String,
		ErrorMessage: errMsg.String,
	}, nil
}

func (s *JobStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM jobs`)
	if err != nil {
		return fmt.Errorf("clear jobs: %w", err)
	}
	return nil
}

func sqlTime(raw string) (time.Time, error) {
	t, err := time.Parse(timeLayout, raw)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("parse time: %w", err)
}
