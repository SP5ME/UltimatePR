package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/packet-radio/ultimatepr/internal/config"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

const updateJobStatusPath = "/var/lib/ultimatepr/update-job.state"

type updateJobStatus struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Stage     string `json:"stage"`
	Progress  string `json:"progress"`
	ExitCode  string `json:"exit_code"`
	UpdatedAt string `json:"updated_at"`
}

func branchReleaseEndpoint(channel string) string {
	return "https://api.github.com/repos/SP5ME/UltimatePR/releases/tags/" + channel + "-latest"
}

func loadUpdateJobStatus() updateJobStatus {
	b, err := os.ReadFile(updateJobStatusPath)
	if err != nil {
		return updateJobStatus{Status: "idle"}
	}
	state := updateJobStatus{Status: "idle"}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		switch strings.TrimSpace(key) {
		case "status":
			state.Status = strings.TrimSpace(value)
		case "message":
			state.Message = strings.TrimSpace(value)
		case "stage":
			state.Stage = strings.TrimSpace(value)
		case "progress":
			state.Progress = strings.TrimSpace(value)
		case "exit_code":
			state.ExitCode = strings.TrimSpace(value)
		case "updated_at":
			state.UpdatedAt = strings.TrimSpace(value)
		}
	}
	if state.Status == "" {
		state.Status = "idle"
	}
	return state
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	channel := c.Application.UpdateChannel
	endpoint := branchReleaseEndpoint(channel)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "UltimatePR-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Nie można połączyć się z GitHub: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// A rolling branch release is briefly absent while GitHub replaces its
		// tag and assets. One delayed retry avoids exposing that publication
		// window as a permanent error in the panel.
		resp.Body.Close()
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			http.Error(w, "Przekroczono czas sprawdzania aktualizacji", 504)
			return
		}
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "UltimatePR-updater")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "Nie można połączyć się z GitHub: "+err.Error(), 502)
			return
		}
		defer resp.Body.Close()
	}
	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("GitHub zwrócił status %d", resp.StatusCode), 502)
		return
	}
	var rel githubRelease
	if json.NewDecoder(resp.Body).Decode(&rel) != nil {
		http.Error(w, "Nieprawidłowa odpowiedź GitHub", 502)
		return
	}
	installed := strings.TrimSpace(s.cfg.Version)
	if installed == "" {
		installed = "development"
	}
	available := rel.TagName
	if rel.Name != "" {
		available = rel.Name
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"installed": installed, "available": available, "channel": channel, "update_available": installed != available})
}

func (s *Server) updateChannel(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Channel string `json:"channel"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil || (q.Channel != "main" && q.Channel != "dev") {
		http.Error(w, "Kanał musi być main albo dev", 400)
		return
	}
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	c.Application.UpdateChannel = q.Channel
	if err = appconfig.SaveModel(s.cfg.ConfigPath, c); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateApply(w http.ResponseWriter, r *http.Request) {
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	launcher := "doas"
	if _, err = exec.LookPath(launcher); err != nil {
		launcher = "sudo"
		if _, err = exec.LookPath(launcher); err != nil {
			http.Error(w, "Aktualizacja wymaga doas albo sudo skonfigurowanego przez install.sh", 503)
			return
		}
	}
	state := loadUpdateJobStatus()
	if state.Status == "running" || state.Status == "queued" {
		http.Error(w, "Aktualizacja już trwa", http.StatusConflict)
		return
	}
	args := []string{"/usr/local/sbin/ultimatepr-update", "--status-file", updateJobStatusPath, c.Application.UpdateChannel}
	cmd := exec.CommandContext(context.Background(), launcher, args...)
	if err = cmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	go func() { _ = cmd.Wait() }()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"started": true, "status_path": filepath.Base(updateJobStatusPath)})
}

func (s *Server) updateJobStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(loadUpdateJobStatus())
}
