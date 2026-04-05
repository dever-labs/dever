package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHasFailures_Empty(t *testing.T) {
	r := Report{}
	if r.HasFailures() {
		t.Fatal("expected no failures in empty report")
	}
}

func TestHasFailures_AllPass(t *testing.T) {
	r := Report{Checks: []Check{
		{Name: "a", Status: "PASS"},
		{Name: "b", Status: "PASS"},
		{Name: "c", Status: "WARN"},
	}}
	if r.HasFailures() {
		t.Fatal("expected no failures when all checks are PASS or WARN")
	}
}

func TestHasFailures_WithFail(t *testing.T) {
	r := Report{Checks: []Check{
		{Name: "a", Status: "PASS"},
		{Name: "b", Status: "FAIL"},
	}}
	if !r.HasFailures() {
		t.Fatal("expected HasFailures to return true when a FAIL check is present")
	}
}

// ── PrintReport ──────────────────────────────────────────────────────────

func TestPrintReport_Human(t *testing.T) {
	report := Report{Checks: []Check{
		{Name: "CLI", Status: "PASS", Detail: "devx dev"},
		{Name: "Runtime: docker", Status: "FAIL", Detail: "docker not found"},
		{Name: "Ports", Status: "WARN", Detail: "port 8080 conflict"},
	}}

	// Capture stdout via a pipe.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	PrintReport(os.Stdout, report, false)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "✓") {
		t.Error("expected PASS icon ✓ in human output")
	}
	if !strings.Contains(output, "✗") {
		t.Error("expected FAIL icon ✗ in human output")
	}
	if !strings.Contains(output, "!") {
		t.Error("expected WARN icon ! in human output")
	}
	if !strings.Contains(output, "docker not found") {
		t.Error("expected detail text in human output")
	}
}

func TestPrintReport_JSON(t *testing.T) {
	report := Report{Checks: []Check{
		{Name: "CLI", Status: "PASS", Detail: "devx dev"},
		{Name: "Runtime: docker", Status: "FAIL"},
	}}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	PrintReport(os.Stdout, report, true)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	var checks []Check
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &checks); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\noutput: %s", err, output)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks in JSON, got %d", len(checks))
	}
	if checks[0].Name != "CLI" {
		t.Errorf("expected first check name 'CLI', got %q", checks[0].Name)
	}
	if checks[1].Status != "FAIL" {
		t.Errorf("expected second check status 'FAIL', got %q", checks[1].Status)
	}
}

func TestPrintReport_JSON_OmitsEmptyDetail(t *testing.T) {
	report := Report{Checks: []Check{
		{Name: "X", Status: "PASS", Detail: ""},
	}}

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	PrintReport(os.Stdout, report, true)
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Detail field has omitempty — must not appear in output when empty.
	if strings.Contains(buf.String(), `"detail"`) {
		t.Error("expected empty detail to be omitted from JSON")
	}
}

// ── doctorIcon ──────────────────────────────────────────────────────────

func TestDoctorIcon(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"PASS", "✓"},
		{"WARN", "!"},
		{"FAIL", "✗"},
		{"", "✗"},
		{"OTHER", "✗"},
	}
	for _, tc := range cases {
		got := doctorIcon(tc.status)
		if got != tc.want {
			t.Errorf("doctorIcon(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
