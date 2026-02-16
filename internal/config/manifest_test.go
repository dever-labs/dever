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
