package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dever-labs/devx/internal/config"
)

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	file := fs.String("file", manifestFile, "Path to devx.yaml to validate")
	_ = fs.Parse(args)

	manifest, err := config.Load(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		return fmt.Errorf("validation failed")
	}

	var issues []string

	if err := config.Validate(manifest); err != nil {
		if ve, ok := err.(*config.ValidationError); ok {
			issues = append(issues, ve.Issues...)
		} else {
			issues = append(issues, err.Error())
		}
	}

	for profName := range manifest.Profiles {
		if err := config.ValidateProfile(manifest, profName); err != nil {
			if ve, ok := err.(*config.ValidationError); ok {
				for _, issue := range ve.Issues {
					issues = append(issues, fmt.Sprintf("[profile:%s] %s", profName, issue))
				}
			} else {
				issues = append(issues, fmt.Sprintf("[profile:%s] %s", profName, err.Error()))
			}
		}
	}

	if err := config.ValidateTools(manifest); err != nil {
		if ve, ok := err.(*config.ValidationError); ok {
			issues = append(issues, ve.Issues...)
		} else {
			issues = append(issues, err.Error())
		}
	}

	if err := config.ValidateSetup(manifest); err != nil {
		if ve, ok := err.(*config.ValidationError); ok {
			issues = append(issues, ve.Issues...)
		} else {
			issues = append(issues, err.Error())
		}
	}

	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "devx.yaml has %d issue(s):\n", len(issues))
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
		return fmt.Errorf("validation failed")
	}

	fmt.Println("devx.yaml is valid ✓")
	return nil
}
