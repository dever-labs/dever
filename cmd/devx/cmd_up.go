package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dever-labs/devx/internal/config"
	"github.com/dever-labs/devx/internal/k8s"
	"github.com/dever-labs/devx/internal/lock"
	"github.com/dever-labs/devx/internal/runtime"
)

func runUp(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	profile := fs.String("profile", "", "Profile to use")
	build := fs.Bool("build", false, "Build images")
	pull := fs.Bool("pull", false, "Always pull images")
	noTelemetry := fs.Bool("no-telemetry", false, "Disable telemetry stack")
	_ = fs.Parse(args)

	manifest, profName, prof, err := loadProfile(*profile)
	if err != nil {
		return err
	}

	rt, err := selectRuntime(ctx)
	if err != nil {
		return err
	}

	lockfile, _ := lock.Load(lockFile)

	if err := ensureDevxDir(); err != nil {
		return err
	}

	runtimeMode := profileRuntime(prof)
	if runtimeMode == "k8s" {
		return runUpK8s(ctx, manifest, profName, prof)
	}

	composePath := filepath.Join(devxDir, composeFile)
	enableTelemetry := !*noTelemetry
	if err := writeCompose(composePath, manifest, profName, prof, lockfile, enableTelemetry); err != nil {
		return err
	}

	if err := rt.Up(ctx, composePath, manifest.Project.Name, runtime.UpOptions{Build: *build, Pull: *pull}); err != nil {
		return err
	}

	if err := waitForHealth(prof); err != nil {
		return err
	}

	if len(prof.Hooks.AfterUp) > 0 {
		fmt.Println("Running afterUp hooks...")
		if err := runHooks(ctx, rt, composePath, manifest.Project.Name, prof.Hooks.AfterUp); err != nil {
			return err
		}
	}

	if err := writeState(state{Profile: profName, Runtime: rt.Name(), Telemetry: enableTelemetry}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write state: %v\n", err)
	}

	fmt.Println("Environment is up")
	printLinks(ctx, rt, composePath, manifest.Project.Name)
	return nil
}

func runUpK8s(ctx context.Context, manifest *config.Manifest, profName string, prof *config.Profile) error {
	output, err := k8s.Render(manifest, profName, prof, "")
	if err != nil {
		return err
	}

	if err := ensureDevxDir(); err != nil {
		return err
	}
	path := filepath.Join(devxDir, k8sFile)
	if err := os.WriteFile(path, []byte(output), 0600); err != nil {
		return err
	}

	if err := k8s.Apply(ctx, path); err != nil {
		return err
	}

	if err := writeState(state{Profile: profName, Runtime: "k8s", Telemetry: false}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write state: %v\n", err)
	}

	fmt.Println("Kubernetes resources applied")
	return nil
}
