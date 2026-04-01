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
	Prompt          string `json:"prompt"`
	ScriptOverride  string `json:"script_override"`
	Voice           string `json:"voice"`
	Language        string `json:"language"`
	Orientation     string `json:"orientation"`
	CustomWidth     int    `json:"custom_width"`
	CustomHeight    int    `json:"custom_height"`
	BackgroundVideo string `json:"background_video"`
}

type Job struct {
	ID           string    `json:"id"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Request      Request   `json:"request"`
	Script       string    `json:"script,omitempty"`
	OutputPath   string    `json:"output_path,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}
