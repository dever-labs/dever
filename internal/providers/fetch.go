package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubAPIBase = "https://api.github.com"
	githubBase    = "https://github.com"
)

// Fetch ensures the provider binary for source@version is present in the
// local cache, downloading it from GitHub releases if necessary.
// version may be "latest" — in that case the latest release tag is resolved
// first and the resolved version is returned.
// Returns the resolved version and the SHA-256 hex digest of the binary.
func Fetch(source, version string) (resolvedVersion, digest string, err error) {
	org, name, err := splitSource(source)
	if err != nil {
		return "", "", err
	}
	repoName := "devx-provider-" + name

	resolvedVersion = version
	if version == "latest" {
		resolvedVersion, err = fetchLatestVersion(org, repoName)
		if err != nil {
			return "", "", fmt.Errorf("resolving latest version for %s: %w", source, err)
		}
	}

	binaryPath, err := BinaryPath(source, resolvedVersion)
	if err != nil {
		return "", "", err
	}

	// Return early if already cached — just verify digest.
	if _, statErr := os.Stat(binaryPath); statErr == nil {
		digest, err = fileSHA256(binaryPath)
		return resolvedVersion, digest, err
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		return "", "", fmt.Errorf("creating provider cache directory: %w", err)
	}

	url := assetURL(org, repoName, resolvedVersion)
	digest, err = downloadFile(url, binaryPath)
	if err != nil {
		// Clean up partial download.
		_ = os.Remove(binaryPath)
		return "", "", fmt.Errorf("downloading provider %s@%s: %w", source, resolvedVersion, err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return "", "", fmt.Errorf("marking provider binary executable: %w", err)
		}
	}

	return resolvedVersion, digest, nil
}

// VerifyDigest checks that the cached binary for source@version matches the
// expected SHA-256 digest. Returns an error if it does not match.
func VerifyDigest(source, version, expected string) error {
	if expected == "" {
		return nil
	}
	binaryPath, err := BinaryPath(source, version)
	if err != nil {
		return err
	}
	actual, err := fileSHA256(binaryPath)
	if err != nil {
		return fmt.Errorf("reading provider binary %s@%s: %w", source, version, err)
	}
	if actual != expected {
		return fmt.Errorf("provider %s@%s digest mismatch: expected %s, got %s", source, version, expected, actual)
	}
	return nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestVersion(org, repoName string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, org, repoName)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d for %s/%s", resp.StatusCode, org, repoName)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// assetURL builds the GitHub release asset URL for the current OS and arch.
// Convention: <org>/<repoName>/releases/download/v<version>/<repoName>-<goos>-<goarch>[.exe]
func assetURL(org, repoName, version string) string {
	v := version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	assetName := fmt.Sprintf("%s-%s-%s", repoName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	return fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", githubBase, org, repoName, v, assetName)
}

func downloadFile(url, dest string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
