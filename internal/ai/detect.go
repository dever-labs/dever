package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const detectSystemPrompt = `You are a devops assistant helping to configure a development environment.
You will be given information about a dependency (e.g. a PostgreSQL database) and the technology
stack of a service that needs to connect to it. Your job is to identify the environment variable
names that the application expects for its connection configuration.

Return ONLY a JSON object mapping environment variable names to their values.
Use the provided connection values in the values (e.g. the actual host, port, password).
Do not include any explanation, markdown formatting, or code blocks — just the raw JSON object.

Example response for a Django app connecting to PostgreSQL:
{"DATABASE_URL": "postgres://myuser:mypassword@db:5432/mydb"}`

// Detect scans serviceDir for framework hints, then calls the LLM to determine
// which environment variable names the application expects for connecting to
// a dependency of the given kind. outputValues contains the resolved connection
// values (host, port, and dep env keys) that should be used in the returned values.
//
// Returns a map of env var name → resolved value ready to inject into the service.
func Detect(ctx context.Context, client *Client, depKind string, outputValues map[string]string, serviceDir string) (map[string]string, error) {
	hints := DetectFrameworks(serviceDir)
	stackDesc := SummariseHints(hints)

	// Build a readable list of available connection values.
	var valueParts []string
	for k, v := range outputValues {
		valueParts = append(valueParts, fmt.Sprintf("  %s = %s", k, v))
	}

	user := fmt.Sprintf(`Service technology stack: %s

Dependency type: %s
Available connection values:
%s

What environment variable names does this application use to connect to the %s dependency?
Return a JSON object mapping each env var name to its value using the connection values above.`,
		stackDesc,
		depKind,
		strings.Join(valueParts, "\n"),
		depKind,
	)

	raw, err := client.complete(ctx, detectSystemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("AI detect call failed: %w", err)
	}

	// Parse the JSON response — strip any accidental markdown fences.
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("AI returned non-JSON response for connection detection: %w\nResponse was: %s", err, raw)
	}
	return result, nil
}
