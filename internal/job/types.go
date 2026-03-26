package job

import "time"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusCompleted Status = "completed"
)

type Request struct {
	Topic       string `json:"topic"`
	Category    string `json:"category"`
	Voice       string `json:"voice"`
	TargetSecs  int    `json:"target_seconds"`
	CountryCode string `json:"country_code"`
}

type Job struct {
	ID           string    `json:"id"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Request      Request   `json:"request"`
	OutputPath   string    `json:"output_path,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}
