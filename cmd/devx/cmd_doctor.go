package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/dever-labs/devx/internal/doctor"
)

func runDoctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fix := fs.Bool("fix", false, "Attempt fixes")
	_ = fs.Parse(args)

	manifest, _, _, _ := loadProfile("")

	report := doctor.Run(ctx, doctor.Options{
		Manifest: manifest,
		Fix:      *fix,
	})

	doctor.PrintReport(os.Stdout, report)
	if report.HasFailures() {
		return errors.New("doctor found failures")
	}
	return nil
}
