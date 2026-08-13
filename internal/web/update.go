package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	appconfig "github.com/packet-radio/ultimatepr/internal/config"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	c, err := appconfig.Load(s.cfg.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	channel := c.Application.UpdateChannel
	endpoint := "https://api.github.com/repos/SP5ME/UltimatePR/releases/tags/" + channel + "-latest"
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
	cmd := exec.CommandContext(r.Context(), launcher, "/usr/local/sbin/ultimatepr-update", c.Application.UpdateChannel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, strings.TrimSpace(string(out))+": "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"started": true, "message": strings.TrimSpace(string(out))})
}
