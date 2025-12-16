package templates

import (
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("02 Jan 2006 15:04")
		},
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			if length <= 3 {
				return s[:length]
			}
			return s[:length-3] + "..."
		},
		"substr": func(s string, start, length int) string {
			if start >= len(s) {
				return ""
			}
			end := min(start+length, len(s))
			return s[start:end]
		},
		"upper": strings.ToUpper,
	}
}

func TestTemplateParsing(t *testing.T) {
	layoutPath := filepath.Join(".", "base.gohtml")
	require.FileExists(t, layoutPath, "base layout is required for all templates")

	partials, err := filepath.Glob(filepath.Join(".", "partials", "*.gohtml"))
	require.NoError(t, err)

	pages, err := filepath.Glob(filepath.Join(".", "*.gohtml"))
	require.NoError(t, err)
	require.NotEmpty(t, pages, "no page templates found")

	for _, page := range pages {
		if filepath.Base(page) == "base.gohtml" {
			continue
		}

		page := page
		t.Run(filepath.Base(page), func(t *testing.T) {
			t.Parallel()

			files := append([]string{layoutPath}, partials...)
			files = append(files, page)

			tmpl, err := template.New(filepath.Base(page)).Funcs(templateFuncMap()).ParseFiles(files...)
			require.NoError(t, err)
			require.NotNil(t, tmpl.Lookup("layout"), "layout template should be available")
			require.NotNil(t, tmpl.Lookup("content"), "content block should be defined by page templates")
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
