package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dever-labs/devx/internal/config"
	"github.com/dever-labs/devx/internal/providers"
)

type providerEntry struct {
	kind    string
	source  string
	version string
}

// collectAllProviders returns de-duplicated provider entries across all profiles.
func collectAllProviders(manifest *config.Manifest) []providerEntry {
	seen := map[string]bool{}
	var entries []providerEntry
	for _, prof := range manifest.Profiles {
		for _, dep := range prof.Deps {
			if dep.Kind == "" || dep.Version == "" {
				continue
			}
			src := depProviderSource(dep)
			key := src + "@" + dep.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, providerEntry{
				kind:    dep.Kind,
				source:  src,
				version: dep.Version,
			})
		}
	}
	return entries
}

func runProviders(ctx context.Context, args []string) error {
	_ = ctx
	if len(args) == 0 {
		return errors.New("providers requires 'install' or 'list'")
	}
	manifest, err := loadManifestOnly()
	if err != nil {
		return err
	}
	switch args[0] {
	case "install":
		for _, entry := range collectAllProviders(manifest) {
			cached, _ := providers.IsCached(entry.source, entry.version)
			if cached {
				fmt.Printf("  ✓ %s@%s (already cached)\n", entry.source, entry.version)
				continue
			}
			fmt.Printf("Fetching provider %q (%s@%s)...\n", entry.kind, entry.source, entry.version)
			resolvedVersion, digest, err := providers.Fetch(entry.source, entry.version)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", entry.kind, err)
				continue
			}
			fmt.Printf("  ✓ %s@%s (sha256:%s)\n", entry.source, resolvedVersion, digest[:12])
		}
	case "list":
		entries := collectAllProviders(manifest)
		if len(entries) == 0 {
			fmt.Println("No providers declared in devx.yaml")
			return nil
		}
		for _, entry := range entries {
			cached, _ := providers.IsCached(entry.source, entry.version)
			status := "not cached"
			if cached {
				status = "cached"
			}
			fmt.Printf("  %-30s %s  [%s]\n", entry.source, entry.version, status)
		}
	default:
		return fmt.Errorf("unknown providers sub-command %q — use 'install' or 'list'", args[0])
	}
	return nil
}
