package setup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var stateFile = ".devx/setup-state.json"

// StepRecord tracks a single setup step that has run successfully.
type StepRecord struct {
	Hash    string `json:"hash"`    // sha256 of name|run|workdir
	LastRun string `json:"lastRun"` // RFC3339 timestamp
}

// State is the full on-disk state persisted to .devx/setup-state.json.
type State struct {
	Steps map[string]StepRecord `json:"steps"`
}

func loadState() *State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return &State{Steps: map[string]StepRecord{}}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return &State{Steps: map[string]StepRecord{}}
	}
	if s.Steps == nil {
		s.Steps = map[string]StepRecord{}
	}
	return &s
}

func saveState(s *State) error {
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0600)
}

func stepHash(name, run, workdir string) string {
	h := sha256.Sum256([]byte(name + "|" + run + "|" + workdir))
	return fmt.Sprintf("%x", h[:8])
}

func hasRunBefore(s *State, name, run, workdir string) bool {
	rec, ok := s.Steps[name]
	if !ok {
		return false
	}
	return rec.Hash == stepHash(name, run, workdir)
}

func markDone(s *State, name, run, workdir string) {
	s.Steps[name] = StepRecord{
		Hash:    stepHash(name, run, workdir),
		LastRun: time.Now().UTC().Format(time.RFC3339),
	}
}
