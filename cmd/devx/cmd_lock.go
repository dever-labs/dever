package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dever-labs/devx/internal/lock"
	"github.com/dever-labs/devx/internal/providers"
	"github.com/dever-labs/devx/internal/runtime"
)

func runLock(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "update" {
		return errors.New("lock requires 'update'")
	}

	manifest, profName, prof, err := loadProfile("")
	if err != nil {
		return err
	}

	rt, err := selectRuntime(ctx)
	if err != nil {
		return err
	}

	resolver, ok := rt.(runtime.DigestResolver)
	if !ok {
		return errors.New("runtime does not support digest resolution")
	}

	lf := lock.New()

	// Pin image digests.
	images, err := collectImages(manifest, profName, prof)
	if err != nil {
		return err
	}
	for _, image := range images {
		digest, err := resolver.ResolveImageDigest(ctx, image)
		if err != nil {
			return fmt.Errorf("lock update failed for %s: %w", image, err)
		}
		lf.Images[image] = digest
	}

	// Pin provider versions and SHA-256 digests from all profiles.
	for _, entry := range collectAllProviders(manifest) {
		fmt.Printf("Pinning provider %q (%s@%s)...\n", entry.kind, entry.source, entry.version)
		resolvedVersion, digest, err := providers.Fetch(entry.source, entry.version)
		if err != nil {
			return fmt.Errorf("lock update failed for provider %q: %w", entry.kind, err)
		}
		lf.Providers[entry.source] = lock.ProviderPin{
			Version: resolvedVersion,
			SHA256:  digest,
		}
		fmt.Printf("  ✓ %s@%s (sha256:%s)\n", entry.source, resolvedVersion, digest[:12])
	}

	return lock.Save(lockFile, lf)
}
