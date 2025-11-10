package templates

import (
	"html/template"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateParsing(t *testing.T) {
	templateDir := "./"

	var templateFiles []string
	err := filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".gohtml" {
			templateFiles = append(templateFiles, path)
		}
		return nil
	})
	require.NoError(t, err)

	tmpl := template.New("test")

	tmpl = tmpl.Funcs(template.FuncMap{
		"formatTime": func(any) string { return "2024-01-01" },
		"eq":         func(a, b any) bool { return a == b },
		"ne":         func(a, b any) bool { return a != b },
		"len":        func(s any) int { return 0 },
		"range":      func(s any) any { return s },
	})

	for _, file := range templateFiles {
		t.Run(file, func(t *testing.T) {
			_, err := tmpl.ParseFiles(file)
			assert.NoError(t, err, "Template %s should parse without errors", file)
		})
	}
}

func TestTemplateSyntax(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		shouldFail bool
	}{
		{
			name:       "valid template",
			template:   `{{define "test"}}<h1>{{.Title}}</h1>{{end}}`,
			shouldFail: false,
		},
		{
			name:       "invalid define inside content",
			template:   `<div>{{define "invalid"}}content{{end}}</div>{{define "test"}}<h1>{{.Title}}</h1>{{end}}`,
			shouldFail: true,
		},
		{
			name:       "unclosed template tag",
			template:   `{{define "test"}}<h1>{{.Title}}</h1>`,
			shouldFail: true,
		},
		{
			name:       "valid nested template",
			template:   `{{define "parent"}}<div>{{template "child" .}}</div>{{end}}{{define "child"}}<span>{{.Content}}</span>{{end}}`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := template.New("test")
			_, err := tmpl.Parse(tt.template)

			if tt.shouldFail {
				assert.Error(t, err, "Template should fail to parse")
			} else {
				assert.NoError(t, err, "Template should parse successfully")
			}
		})
	}
}
