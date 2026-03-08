// Package providers manages provider plugins that supply behavioural logic
// (health checks, compose fragments) for dep kinds. Providers are downloaded
// from GitHub releases on demand and cached under ~/.devx/providers/.
package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ProviderMeta is the metadata returned by a provider binary's describe
// subcommand. DefaultImage is the OCI image the provider was built and tested
// against — devx uses it when a dep declares a kind but omits its own image.
// Outputs lists the named values the provider can supply for connection string
// injection (e.g. "host", "port", "user", "password", "dbname").
type ProviderMeta struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	DefaultImage string   `json:"defaultImage"`
	Description  string   `json:"description,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
}

// Describe calls the provider binary's describe subcommand and returns its
// metadata. Returns (nil, nil) if the binary is not yet cached so callers can
// distinguish "not installed" from a real error.
func Describe(source, version string) (*ProviderMeta, error) {
	binaryPath, err := BinaryPath(source, version)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, nil
	}

	out, err := exec.Command(binaryPath, "describe").Output()
	if err != nil {
		return nil, fmt.Errorf("provider %s describe: %w", source, err)
	}

	var meta ProviderMeta
	if err := json.Unmarshal(out, &meta); err != nil {
		return nil, fmt.Errorf("provider %s describe returned invalid JSON: %w", source, err)
	}
	return &meta, nil
}

// DepInput is the JSON payload sent to a provider binary on stdin.
type DepInput struct {
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	Ports  []string          `json:"ports,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
	Volume string            `json:"volume,omitempty"`
}

// ComposeFragment is the partial compose service configuration a provider
// returns from its render-compose subcommand. Only fields present in the
// output are merged; the project's own dep config takes precedence.
type ComposeFragment struct {
	Healthcheck *Healthcheck `json:"healthcheck,omitempty"`
}

// Healthcheck mirrors the compose healthcheck structure.
type Healthcheck struct {
	Test     []string `json:"test"`
	Interval string   `json:"interval,omitempty"`
	Retries  int      `json:"retries,omitempty"`
}

// CacheDir returns the root directory used to cache provider binaries.
// On all platforms this is $HOME/.devx/providers/.
func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".devx", "providers"), nil
}

// BinaryPath returns the expected filesystem path for a cached provider binary.
// source is in "org/name" format (e.g. "devx-labs/postgres").
func BinaryPath(source, version string) (string, error) {
	org, name, err := splitSource(source)
	if err != nil {
		return "", err
	}
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}
	binaryName := "devx-provider-" + name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	return filepath.Join(cacheDir, org, name, version, binaryName), nil
}

// IsCached reports whether a provider binary is present in the local cache.
func IsCached(source, version string) (bool, error) {
	path, err := BinaryPath(source, version)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	return err == nil, nil
}

// InvokeRenderCompose calls the provider binary's render-compose subcommand,
// passing dep as JSON on stdin. It returns the provider's ComposeFragment.
// Returns (nil, nil) if the binary is not cached — callers should treat a nil
// fragment as "no provider enhancements" and continue normally.
func InvokeRenderCompose(source, version string, dep DepInput) (*ComposeFragment, error) {
	binaryPath, err := BinaryPath(source, version)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return nil, nil
	}

	input, err := json.Marshal(dep)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binaryPath, "render-compose")
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("provider %s render-compose: %w", source, err)
	}

	var fragment ComposeFragment
	if err := json.Unmarshal(out, &fragment); err != nil {
		return nil, fmt.Errorf("provider %s returned invalid JSON: %w", source, err)
	}
	return &fragment, nil
}

// splitSource splits "org/name" into (org, name, nil) or returns an error.
func splitSource(source string) (org, name string, err error) {
	parts := strings.SplitN(source, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("provider source %q must be in org/name format", source)
	}
	return parts[0], parts[1], nil
}

// ResolveOutputValues builds the template variable map for a dep's connect
// block. The map contains:
//   - "host" → depName (the service hostname within the compose network)
//   - "port" → the first container-side port from depPorts (e.g. "5432" from "5432:5432")
//   - one entry per key in depEnv (so templates can use ${POSTGRES_PASSWORD} etc.)
func ResolveOutputValues(depName string, depPorts []string, depEnv map[string]string) map[string]string {
	vals := map[string]string{"host": depName}

	if len(depPorts) > 0 {
		// Take the container-side port (right of the colon, or the whole value if no colon).
		p := depPorts[0]
		if idx := strings.LastIndex(p, ":"); idx >= 0 {
			p = p[idx+1:]
		}
		vals["port"] = p
	}

	for k, v := range depEnv {
		vals[k] = v
	}

	return vals
}
