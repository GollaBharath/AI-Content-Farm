package storage

import (
	"sync"

	"github.com/Gollabharath/ai-content-farm/internal/job"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]job.Job
}

func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]job.Job)}
}

func (s *JobStore) Save(j job.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

func (s *JobStore) Get(id string) (job.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *JobStore) List() []job.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]job.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		result = append(result, j)
	}
	return result
}
