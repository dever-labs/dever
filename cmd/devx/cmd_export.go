package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dever-labs/devx/internal/k8s"
	"github.com/dever-labs/devx/internal/lock"
)

func runExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	format := fs.String("format", "", "Output format: compose|k8s|helm|terraform")
	profile := fs.String("profile", "", "Profile to export")
	outDir := fs.String("out", ".", "Output directory")
	_ = fs.Parse(args)

	if *format == "" {
		return errors.New("--format is required (compose|k8s|helm|terraform)")
	}

	manifest, profName, prof, err := loadProfile(*profile)
	if err != nil {
		return err
	}

	switch *format {
	case "compose":
		lockfile, _ := lock.Load(lockFile)
		output, err := buildCompose(manifest, profName, prof, lockfile, false)
		if err != nil {
			return err
		}
		outPath := filepath.Join(*outDir, "compose.yaml")
		if err := writeTextFile(outPath, output); err != nil {
			return err
		}
		fmt.Printf("Exported compose to %s\n", outPath)

	case "k8s":
		prof = resolveDepImages(prof)
		prof = resolveConnections(manifest, prof)
		output, err := k8s.Render(manifest, profName, prof, "")
		if err != nil {
			return err
		}
		outPath := filepath.Join(*outDir, "k8s.yaml")
		if err := writeTextFile(outPath, output); err != nil {
			return err
		}
		fmt.Printf("Exported k8s manifests to %s\n", outPath)

	case "helm":
		return errors.New("helm export not yet implemented")

	case "terraform":
		return errors.New("terraform export not yet implemented")

	default:
		return fmt.Errorf("unknown format %q — use compose, k8s, helm, or terraform", *format)
	}
	return nil
}

func writeTextFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
