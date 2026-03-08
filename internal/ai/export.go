package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dever-labs/devx/internal/config"
	"gopkg.in/yaml.v3"
)

const exportSystemPrompt = `You are a senior devops engineer generating production-ready deployment configuration.
You will be given a devx.yaml profile describing a set of services and their dependencies.
Your job is to generate complete, production-grade deployment files for the requested format.

Production-grade means:
- Resource requests and limits for all containers
- Liveness and readiness probes where applicable
- Secrets for sensitive environment variables (passwords, keys)
- Ingress / load balancer configuration for externally exposed ports
- Health checks and restart policies
- Horizontal Pod Autoscaler (HPA) for stateless services (k8s/helm)
- Persistent volume claims for stateful deps (k8s/helm)
- Named volumes and restart policies for compose

Return a JSON object where each key is a filename (relative path) and each value is the
complete file content as a string. Do not include any explanation or markdown — just the JSON.

Example response shape:
{"docker-compose.prod.yml": "version: '3.9'\nservices:\n  ...", "README.md": "# Deployment\n..."}`

// GenerateDeployment calls the LLM to produce production deployment files for
// the given format. It serialises the manifest and profile to YAML as context,
// detects framework hints for each service with a build context, and asks the
// LLM to generate complete, production-grade output.
//
// Returns a map of relative filename → file content.
// Supported formats: compose, k8s, helm, terraform (others accepted — AI decides).
func GenerateDeployment(ctx context.Context, client *Client, manifest *config.Manifest, profile *config.Profile, profileName, format string) (map[string]string, error) {
	// Serialise the profile for context.
	profileYAML, err := yaml.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("serialising profile: %w", err)
	}

	// Collect framework hints per service.
	var hintLines []string
	for name, svc := range profile.Services {
		if svc.Build == nil || svc.Build.Context == "" {
			continue
		}
		hints := DetectFrameworks(svc.Build.Context)
		if len(hints) > 0 {
			hintLines = append(hintLines, fmt.Sprintf("  %s: %s", name, SummariseHints(hints)))
		}
	}

	frameworkSection := ""
	if len(hintLines) > 0 {
		frameworkSection = "\nDetected technology stacks:\n" + strings.Join(hintLines, "\n")
	}

	user := fmt.Sprintf(`Project: %s
Profile: %s
Target format: %s
%s
Profile configuration (devx.yaml):
%s

Generate production-ready %s deployment files for this project.
Return a JSON object mapping filename to file content.`,
		manifest.Project.Name,
		profileName,
		format,
		frameworkSection,
		string(profileYAML),
		format,
	)

	raw, err := client.complete(ctx, exportSystemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("AI export call failed: %w", err)
	}

	// Strip any markdown fences the model may have added.
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var files map[string]string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil, fmt.Errorf("AI returned non-JSON response for deployment generation: %w\nResponse was: %s", err, raw)
	}
	return files, nil
}
