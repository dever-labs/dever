package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dever-labs/devx/internal/lock"
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

	return lock.Save(lockFile, lf)
}
