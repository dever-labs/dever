package config

import "testing"

func TestValidateManifest(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := Validate(m); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err != nil {
		t.Fatalf("profile validation failed: %v", err)
	}
}

func TestValidateManifestErrors(t *testing.T) {
	data := []byte(`version: 2
project:
  name: ""
  defaultProfile: ""
profiles: {}
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := Validate(m); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProfileRuntime(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: bad
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected runtime validation error")
	}
}

func TestValidateProfileDependsOnMissing(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
        dependsOn:
          - db
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected error for missing dependsOn target")
	}
}

func TestValidateProfileDepMissingImage(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
    deps:
      cache:
        kind: redis
`)
	// kind is set but no version — should fail because version is required when kind is set.
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected error for dep kind without version")
	}
}

func TestValidateProfileDepNoKindRequiresImage(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
    deps:
      db:
        ports:
          - "1433:1433"
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// No kind and no image — must fail.
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected error for dep with neither kind nor image")
	}
}

func TestValidateProfileDepKindMissingVersion(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
    deps:
      cache:
        kind: redis
        image: redis:7
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// kind is set but version is missing — must fail.
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected error for dep kind without version")
	}
}

func TestValidateProfileDepWithKindAndVersion(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
    deps:
      cache:
        kind: redis
        version: "7.2.0"
        image: redis:7
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := Validate(m); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err != nil {
		t.Fatalf("expected no error when kind+version are declared: %v", err)
	}
}

func TestValidateProfileDepNoKind(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
    deps:
      db:
        image: mcr.microsoft.com/mssql/server:2022-latest
        ports:
          - "1433:1433"
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Dep without kind is valid — no provider needed.
	if err := ValidateProfile(m, "local"); err != nil {
		t.Fatalf("expected no error for dep without kind: %v", err)
	}
}

func TestProfileByName(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	prof, err := ProfileByName(m, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prof.Runtime != "compose" {
		t.Fatalf("expected runtime compose, got %q", prof.Runtime)
	}

	if _, err := ProfileByName(m, "missing"); err == nil {
		t.Fatalf("expected error for missing profile")
	}
}

func TestValidateConnectServiceExists(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
    deps:
      db:
        kind: postgres
        version: "16.1.0"
        image: postgres:16
        connect:
          - service: missing-service
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatalf("expected error for connect referencing non-existent service")
	}
}

func TestValidateConnectValid(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
    deps:
      db:
        kind: postgres
        version: "16.1.0"
        image: postgres:16
        connect:
          - service: api
            env:
              DATABASE_URL: "postgres://postgres:password@${host}:${port}/appdb"
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := ValidateProfile(m, "local"); err != nil {
		t.Fatalf("expected no error for valid connect block: %v", err)
	}
}

func TestValidateAIConfig(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
ai:
  provider: openai
  model: gpt-4o-mini
profiles:
  local:
    services:
      api:
        image: nginx:alpine
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := Validate(m); err != nil {
		t.Fatalf("expected no error for valid AI config: %v", err)
	}
}

func TestValidateAIConfigMissingModel(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
ai:
  provider: openai
profiles:
  local:
    services:
      api:
        image: nginx:alpine
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if err := Validate(m); err == nil {
		t.Fatalf("expected error for AI config missing model")
	}
}
