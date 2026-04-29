// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package htmltext_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/mailservice/htmltext"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain heading and paragraph",
			input:    `<html><body><h1>Hello</h1><p>World</p></body></html>`,
			expected: "Hello\nWorld\n",
		},
		{
			name:     "head block skipped",
			input:    `<html><head><title>Page Title</title></head><body><p>Body text</p></body></html>`,
			expected: "Body text\n",
		},
		{
			name:     "style block skipped",
			input:    `<html><head><style>body { color: red; }</style></head><body><p>Visible</p></body></html>`,
			expected: "Visible\n",
		},
		{
			name:     "script block skipped",
			input:    `<html><body><script>alert('xss')</script><p>Visible</p></body></html>`,
			expected: "Visible\n",
		},
		{
			name:     "link rendered as text ( url )",
			input:    `<p>Click <a href="https://example.com">here</a> to continue.</p>`,
			expected: "Click here ( https://example.com ) to continue.\n",
		},
		{
			name:     "link with go template href preserved",
			input:    `<a href="{{ .Data.ResetLink }}">Reset Password</a>`,
			expected: "Reset Password ( {{ .Data.ResetLink }} )\n",
		},
		{
			name:     "image alt text extracted",
			input:    `<img src="logo.png" alt="{{ .BrandName }} Logo">`,
			expected: "{{ .BrandName }} Logo\n",
		},
		{
			name:     "image inside link uses alt as link text",
			input:    `<a href="{{ .HomepageURL }}"><img src="logo.png" alt="Storj Logo"></a>`,
			expected: "Storj Logo ( {{ .HomepageURL }} )\n",
		},
		{
			name:     "br produces newline",
			input:    `<p>Line one<br>Line two</p>`,
			expected: "Line one\nLine two\n",
		},
		{
			name:     "go template conditionals preserved",
			input:    "{{ if .SourceCodeURL }}\n<a href=\"{{ .SourceCodeURL }}\">GitHub</a>\n{{ end }}",
			expected: "{{ if .SourceCodeURL }}\nGitHub ( {{ .SourceCodeURL }} )\n{{ end }}\n",
		},
		{
			name:     "blank lines collapsed",
			input:    "<p>First</p><p></p><p>Second</p>",
			expected: "First\nSecond\n",
		},
		{
			name:     "link with empty href not formatted as link",
			input:    `<a href="">plain text</a>`,
			expected: "plain text\n",
		},
		{
			name:     "link with hash href not formatted as link",
			input:    `<a href="#">plain text</a>`,
			expected: "plain text\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, htmltext.Convert(strings.NewReader(tt.input)))
		})
	}
}
