package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type envelope struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	Command       string   `json:"command"`
	Data          any      `json:"data,omitempty"`
	Warnings      []string `json:"warnings"`
	NextActions   []string `json:"next_actions"`
}

func emitJSON(w io.Writer, command string, data any, warnings, next []string) error {
	return emitJSONStatus(w, true, command, data, warnings, next)
}

func emitJSONStatus(w io.Writer, ok bool, command string, data any, warnings, next []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	if next == nil {
		next = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(envelope{
		SchemaVersion: 1,
		OK:            ok,
		Command:       command,
		Data:          data,
		Warnings:      warnings,
		NextActions:   next,
	}); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}
