package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dever-labs/devx/internal/runtime"
	"github.com/dever-labs/devx/internal/ui"
)

func runStatus(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	outputJSON := fs.Bool("json", false, "Emit status as JSON")
	_ = fs.Parse(args)

	manifest, profName, prof, err := loadProfile("")
	if err != nil {
		return err
	}

	if profileRuntime(prof) == "k8s" {
		return errors.New("status for k8s runtime is not supported yet")
	}

	rt, err := selectRuntime(ctx)
	if err != nil {
		return err
	}

	enableTelemetry := telemetryFromState()
	composePath := filepath.Join(devxDir, composeFile)
	if err := ensureDevxDir(); err != nil {
		return err
	}
	if err := writeCompose(composePath, manifest, profName, prof, nil, enableTelemetry); err != nil {
		return err
	}

	statuses, err := rt.Status(ctx, composePath, manifest.Project.Name)
	if err != nil {
		return err
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	if *outputJSON {
		return printStatusJSON(statuses)
	}

	headers := []string{"Service", "State", "Health", "Ports"}
	rows := make([][]string, 0, len(statuses))
	for _, st := range statuses {
		rows = append(rows, []string{st.Name, st.State, st.Health, st.Ports})
	}
	ui.PrintTable(os.Stdout, headers, rows)
	return nil
}

func printStatusJSON(statuses []runtime.ServiceStatus) error {
	type jsonStatus struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Health string `json:"health"`
		Ports  string `json:"ports"`
	}
	out := make([]jsonStatus, len(statuses))
	for i, s := range statuses {
		out[i] = jsonStatus{Name: s.Name, State: s.State, Health: s.Health, Ports: s.Ports}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
