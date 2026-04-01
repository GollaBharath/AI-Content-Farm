package tts

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type ComposeManager struct {
	projectDir string
	service    string
}

func NewComposeManager(projectDir, service string) *ComposeManager {
	return &ComposeManager{
		projectDir: strings.TrimSpace(projectDir),
		service:    strings.TrimSpace(service),
	}
}

func (m *ComposeManager) SyncPiperEnabled(ctx context.Context, enabled bool) error {
	if m == nil || m.service == "" {
		return nil
	}

	args := []string{}
	if enabled {
		args = append(args, "start", m.service)
	} else {
		args = append(args, "stop", m.service)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	if strings.TrimSpace(m.projectDir) != "" {
		cmd.Dir = m.projectDir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if enabled && strings.Contains(strings.ToLower(trimmed), "no such container") {
			return fmt.Errorf("docker container %q was not found; create the local tts container first", m.service)
		}
		if trimmed == "" {
			return fmt.Errorf("docker %v failed: %w", args, err)
		}
		return fmt.Errorf("docker %v failed: %w (%s)", args, err, trimmed)
	}
	return nil
}

func ResolveProjectDir(projectDir string) string {
	p := strings.TrimSpace(projectDir)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
