package config

import "testing"

func TestValidateHooks_Valid(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    hooks:
      afterUp:
        - exec: "migrate up"
          service: api
        - run: "./scripts/seed.sh"
      beforeDown:
        - exec: "migrate down"
          service: api
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := ValidateProfile(m, "local"); err != nil {
		t.Fatalf("expected valid hooks, got: %v", err)
	}

	afterUp := m.Profiles["local"].Hooks.AfterUp
	if len(afterUp) != 2 {
		t.Fatalf("expected 2 afterUp hooks, got %d", len(afterUp))
	}
	if afterUp[0].Exec != "migrate up" || afterUp[0].Service != "api" {
		t.Errorf("unexpected hook[0]: %+v", afterUp[0])
	}
	if afterUp[1].Run != "./scripts/seed.sh" {
		t.Errorf("unexpected hook[1]: %+v", afterUp[1])
	}
}

func TestValidateHooks_ExecMissingService(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    hooks:
      afterUp:
        - exec: "migrate up"
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatal("expected error: exec hook missing service")
	}
}

func TestValidateHooks_BothExecAndRun(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    hooks:
      afterUp:
        - exec: "migrate up"
          service: api
          run: "./scripts/seed.sh"
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatal("expected error: hook sets both exec and run")
	}
}

func TestValidateHooks_EmptyHook(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    hooks:
      afterUp:
        - service: api
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatal("expected error: hook sets neither exec nor run")
	}
}

func TestValidateHooks_RunWithService(t *testing.T) {
	data := []byte(`version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    runtime: compose
    hooks:
      afterUp:
        - run: "./scripts/seed.sh"
          service: api
    services:
      api:
        image: nginx:alpine
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := ValidateProfile(m, "local"); err == nil {
		t.Fatal("expected error: run hook should not set service")
	}
}
