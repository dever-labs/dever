package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dever-labs/devx/internal/compose"
	"github.com/dever-labs/devx/internal/config"
	"github.com/dever-labs/devx/internal/graph"
	"github.com/dever-labs/devx/internal/lock"
	"github.com/dever-labs/devx/internal/runtime"
	"github.com/dever-labs/devx/internal/runtime/docker"
	"github.com/dever-labs/devx/internal/runtime/podman"
)

func loadProfile(profile string) (*config.Manifest, string, *config.Profile, error) {
	manifest, err := config.Load(manifestFile)
	if err != nil {
		return nil, "", nil, err
	}

	if err := config.Validate(manifest); err != nil {
		return nil, "", nil, err
	}

	profName := profile
	if profName == "" {
		profName = manifest.Project.DefaultProfile
	}
	if profName == "" {
		return nil, "", nil, fmt.Errorf("no profile specified and no defaultProfile set in manifest")
	}

	prof, err := config.ProfileByName(manifest, profName)
	if err != nil {
		return nil, "", nil, err
	}

	if err := config.ValidateProfile(manifest, profName); err != nil {
		return nil, "", nil, err
	}

	return manifest, profName, prof, nil
}

func writeCompose(path string, manifest *config.Manifest, profName string, prof *config.Profile, lockfile *lock.Lockfile, enableTelemetry bool) error {
	composed, err := buildCompose(manifest, profName, prof, lockfile, enableTelemetry)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(composed), 0600); err != nil {
		return err
	}

	assets := compose.TelemetryAssets(enableTelemetry)
	if len(assets) == 0 {
		return nil
	}

	baseDir := filepath.Dir(path)
	for _, asset := range assets {
		assetPath := filepath.Join(baseDir, asset.Path)
		if err := os.MkdirAll(filepath.Dir(assetPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(assetPath, asset.Content, 0644); err != nil {
			return err
		}
	}

	return nil
}

func buildCompose(manifest *config.Manifest, profName string, prof *config.Profile, lockfile *lock.Lockfile, enableTelemetry bool) (string, error) {
	g, err := graph.Build(prof)
	if err != nil {
		return "", err
	}
	if _, err := graph.TopoSort(g); err != nil {
		return "", err
	}

	rewrite := compose.RewriteOptions{
		RegistryPrefix: manifest.Registry.Prefix,
		Lockfile:       lockfile,
	}

	return compose.Render(manifest, profName, prof, rewrite, enableTelemetry)
}

func profileRuntime(prof *config.Profile) string {
	if prof == nil || prof.Runtime == "" {
		return "compose"
	}
	return prof.Runtime
}

func ensureDevxDir() error {
	return os.MkdirAll(devxDir, 0755)
}

func ensureGitignore() error {
	path := ".gitignore"
	entry := ".devx/"

	if !fileExists(path) {
		return os.WriteFile(path, []byte(entry+"\n"), 0644)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if strings.Contains(string(data), entry) {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("\n" + entry + "\n")
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeState(s state) error {
	if err := ensureDevxDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(devxDir, stateFile), data, 0600)
}

func readState() *state {
	data, err := os.ReadFile(filepath.Join(devxDir, stateFile))
	if err != nil {
		return nil
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

func telemetryFromState() bool {
	st := readState()
	if st == nil {
		return true
	}
	return st.Telemetry
}

func streamLogs(reader io.Reader, jsonOut bool) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if !jsonOut {
			fmt.Println(line)
			continue
		}
		entry := map[string]string{"line": line}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return scanner.Err()
}

func waitForHealth(profile *config.Profile) error {
	if profile == nil {
		return nil
	}

	type check struct {
		name string
		url  string
	}

	var checks []check
	for name, svc := range profile.Services {
		if svc.Health == nil || svc.Health.HttpGet == "" {
			continue
		}
		checks = append(checks, check{name: name, url: svc.Health.HttpGet})
	}

	if len(checks) == 0 {
		return nil
	}

	deadline := time.Now().Add(2 * time.Minute)
	pending := map[string]string{}
	for _, c := range checks {
		pending[c.name] = c.url
	}

	for len(pending) > 0 && time.Now().Before(deadline) {
		for name, url := range pending {
			if checkHTTP(url) {
				delete(pending, name)
			}
		}
		if len(pending) == 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if len(pending) > 0 {
		var parts []string
		for name, url := range pending {
			parts = append(parts, fmt.Sprintf("%s (%s)", name, url))
		}
		return fmt.Errorf("health checks failed: %s", strings.Join(parts, ", "))
	}

	return nil
}

func checkHTTP(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func collectImages(manifest *config.Manifest, profileName string, prof *config.Profile) ([]string, error) {
	composed, err := buildCompose(manifest, profileName, prof, nil, true)
	if err != nil {
		return nil, err
	}

	imgs, err := compose.CollectImages([]byte(composed))
	if err != nil {
		return nil, err
	}

	return imgs, nil
}

func selectRuntime(ctx context.Context) (runtime.Runtime, error) {
	d := docker.New()
	if ok, _ := d.Detect(ctx); ok {
		return d, nil
	}
	p := podman.New()
	if ok, _ := p.Detect(ctx); ok {
		return p, nil
	}
	return nil, runtime.ErrNoRuntime
}
