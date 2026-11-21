package database

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed scripts/*
var databaseFS embed.FS

// renderScript executes an embedded shell script template with the given data.
func renderScript(name string, data any) (string, error) {
	tpl, err := template.ParseFS(databaseFS, "scripts/"+name)
	if err != nil {
		return "", fmt.Errorf("failed to parse script %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render script %s: %w", name, err)
	}
	return buf.String(), nil
}
