package main

import (
	"testing"
	"github.com/dever-labs/devx/internal/config"
)

func TestLoadProfile_DefaultProfile(t *testing.T) {
	manifest := &config.Manifest{
		Project: config.Project{
			DefaultProfile: "local",
		},
		Profiles: map[string]*config.Profile{
			"local": {},
		},
	}

	// Simulate config.Load and config.Validate
	configLoad := func(string) (*config.Manifest, error) { return manifest, nil }
	configValidate := func(*config.Manifest) error { return nil }
	configProfileByName := func(*config.Manifest, string) (*config.Profile, error) { return manifest.Profiles["local"], nil }
	configValidateProfile := func(*config.Manifest, string) error { return nil }

	// Patch functions
	oldLoad := config.Load
	oldValidate := config.Validate
	oldProfileByName := config.ProfileByName
	oldValidateProfile := config.ValidateProfile
	config.Load = configLoad
	config.Validate = configValidate
	config.ProfileByName = configProfileByName
	config.ValidateProfile = configValidateProfile
	defer func() {
		config.Load = oldLoad
		config.Validate = oldValidate
		config.ProfileByName = oldProfileByName
		config.ValidateProfile = oldValidateProfile
	}()

	_, profName, _, err := loadProfile("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if profName != "local" {
		t.Fatalf("expected profile 'local', got '%s'", profName)
	}
}

func TestLoadProfile_NoDefaultProfile(t *testing.T) {
	manifest := &config.Manifest{
		Project: config.Project{},
		Profiles: map[string]*config.Profile{
			"local": {},
		},
	}
	configLoad := func(string) (*config.Manifest, error) { return manifest, nil }
	configValidate := func(*config.Manifest) error { return nil }
	configProfileByName := func(*config.Manifest, string) (*config.Profile, error) { return manifest.Profiles["local"], nil }
	configValidateProfile := func(*config.Manifest, string) error { return nil }
	oldLoad := config.Load
	oldValidate := config.Validate
	oldProfileByName := config.ProfileByName
	oldValidateProfile := config.ValidateProfile
	config.Load = configLoad
	config.Validate = configValidate
	config.ProfileByName = configProfileByName
	config.ValidateProfile = configValidateProfile
	defer func() {
		config.Load = oldLoad
		config.Validate = oldValidate
		config.ProfileByName = oldProfileByName
		config.ValidateProfile = oldValidateProfile
	}()

	_, _, _, err := loadProfile("")
	if err == nil {
		t.Fatalf("expected error when no defaultProfile is set")
	}
}
