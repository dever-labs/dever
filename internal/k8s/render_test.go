package k8s

import (
	"strings"
	"testing"

	"github.com/dever-labs/devx/internal/config"
)

func TestRenderK8s(t *testing.T) {
	manifest := &config.Manifest{
		Version: 1,
		Project: config.Project{Name: "my-app", DefaultProfile: "local"},
	}
	profile := &config.Profile{
		Services: map[string]config.Service{
			"api": {Image: "nginx:alpine", Ports: []string{"8080:80"}},
		},
		Deps: map[string]config.Dep{
			"db": {Kind: "postgres", Version: "16", Ports: []string{"5432:5432"}},
		},
	}

	out, err := Render(manifest, "local", profile, "")
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(out, "kind: Deployment") {
		t.Fatalf("expected deployment in output")
	}
	if !strings.Contains(out, "kind: Service") {
		t.Fatalf("expected service in output")
	}
	if !strings.Contains(out, "nginx:alpine") {
		t.Fatalf("expected service image in output")
	}
	if !strings.Contains(out, "postgres:16") {
		t.Fatalf("expected dep image in output")
	}
}
