package main

import (
	"flag"
	"fmt"
	"os"
)

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	_ = fs.Parse(args)

	if fileExists(manifestFile) {
		return fmt.Errorf("%s already exists", manifestFile)
	}

	stub := "version: 1\n\nproject:\n  name: my-app\n  defaultProfile: local\n\nprofiles:\n  local:\n    services:\n      api:\n        build:\n          context: ./api\n          dockerfile: Dockerfile\n        ports:\n          - \"8080:8080\"\n        env:\n          ASPNETCORE_ENVIRONMENT: Development\n        dependsOn: [db]\n        health:\n          httpGet: \"http://localhost:8080/health\"\n          interval: 5s\n          retries: 30\n\n    deps:\n      db:\n        kind: postgres\n        version: \"16\"\n        env:\n          POSTGRES_PASSWORD: postgres\n        ports: [\"5432:5432\"]\n        volume: \"db-data:/var/lib/postgresql/data\"\n"

	if err := os.WriteFile(manifestFile, []byte(stub), 0644); err != nil {
		return err
	}

	if err := ensureDevxDir(); err != nil {
		return err
	}

	if err := ensureGitignore(); err != nil {
		return err
	}

	fmt.Println("Initialized devx.yaml")
	return nil
}
