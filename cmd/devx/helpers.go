package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dever-labs/devx/internal/compose"
	"github.com/dever-labs/devx/internal/config"
	"github.com/dever-labs/devx/internal/graph"
	"github.com/dever-labs/devx/internal/lock"
	devxruntime "github.com/dever-labs/devx/internal/runtime"
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

func selectRuntime(ctx context.Context) (devxruntime.Runtime, error) {
	d := docker.New()
	if ok, _ := d.Detect(ctx); ok {
		return d, nil
	}
	p := podman.New()
	if ok, _ := p.Detect(ctx); ok {
		return p, nil
	}
	return nil, devxruntime.ErrNoRuntime
}

// printLinks queries the running stack for actual host-port bindings and prints
// http://localhost:<port> for every published port. Using the runtime (not the
// compose YAML) ensures randomly-assigned ports are reflected correctly.
func printLinks(ctx context.Context, rt devxruntime.Runtime, composePath, projectName string) {
	statuses, err := rt.Status(ctx, composePath, projectName)
	if err != nil {
		return
	}

	type link struct {
		label string
		url   string
	}
	var links []link
	seen := map[string]bool{}

	for _, svc := range statuses {
		label := serviceLabel(svc.Name)
		for _, pub := range svc.Publishers {
			if pub.PublishedPort == 0 {
				continue
			}
			url := fmt.Sprintf("http://localhost:%d", pub.PublishedPort)
			key := label + url
			if seen[key] {
				continue
			}
			seen[key] = true
			links = append(links, link{label: label, url: url})
		}
	}

	if len(links) == 0 {
		return
	}

	sort.Slice(links, func(i, j int) bool { return links[i].label < links[j].label })

	fmt.Println("\nAvailable services:")
	maxLen := 0
	for _, l := range links {
		if len(l.label) > maxLen {
			maxLen = len(l.label)
		}
	}
	for _, l := range links {
		fmt.Printf("  %-*s  %s\n", maxLen, l.label, l.url)
	}
}

// wellKnownLabels maps telemetry service name suffixes to display labels.
var wellKnownLabels = map[string]string{
	"grafana":     "Grafana",
	"prometheus":  "Prometheus",
	"loki":        "Loki",
	"cadvisor":    "cAdvisor",
	"alloy":       "Alloy",
	"docker-meta": "Docker Meta",
}

func serviceLabel(name string) string {
	lower := strings.ToLower(name)
	// Strip common telemetry prefix
	stripped := strings.TrimPrefix(lower, "devx-telemetry-")
	if label, ok := wellKnownLabels[stripped]; ok {
		return label
	}
	// Title-case the service name for user services
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// runHooks executes a slice of lifecycle hooks sequentially, stopping on first error.
// exec hooks run a command inside a running container; run hooks run a host shell command.
func runHooks(ctx context.Context, rt devxruntime.Runtime, composePath, projectName string, hooks []config.Hook) error {
	for i, h := range hooks {
		if h.Exec != "" {
			fmt.Printf("  [hook %d] exec in %s: %s\n", i+1, h.Service, h.Exec)
			cmd := strings.Fields(h.Exec)
			code, err := rt.Exec(ctx, composePath, projectName, h.Service, cmd)
			if err != nil {
				return fmt.Errorf("hook %d exec failed: %w", i+1, err)
			}
			if code != 0 {
				return fmt.Errorf("hook %d exec exited with code %d", i+1, code)
			}
		} else {
			fmt.Printf("  [hook %d] run: %s\n", i+1, h.Run)
			if err := runShellCommand(h.Run); err != nil {
				return fmt.Errorf("hook %d run failed: %w", i+1, err)
			}
		}
	}
	return nil
}

// runShellCommand runs a command string via the system shell with stdout/stderr inherited.
func runShellCommand(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
