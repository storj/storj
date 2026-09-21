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
			name:     "unclosed head keeps emitting template actions",
			input:    `{{ define "head" }}<html><head><style>a{}</style>{{ end }}`,
			expected: "{{ define \"head\" }}{{ end }}\n",
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
			name:     "aria-hidden content skipped",
			input:    `<a href="https://example.com">One</a><span aria-hidden="true"> &middot; </span><a href="https://example.com/two">Two</a>`,
			expected: "One ( https://example.com )Two ( https://example.com/two )\n",
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

func TestHiddenVoidElements(t *testing.T) {
	voidTags := []string{"area", "base", "basefont", "bgsound", "br", "col", "embed", "frame",
		"hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr"}
	for _, tag := range voidTags {
		t.Run(tag+" starts hidden", func(t *testing.T) {
			input := `<p>Before</p><` + tag + ` aria-hidden="true"><p>After</p>`
			require.Equal(t, "Before\nAfter\n", htmltext.Convert(strings.NewReader(input)))
		})
	}
	for _, tag := range voidTags {
		t.Run(tag, func(t *testing.T) {
			input := `<div style="display:none"><span><` + tag + `>hidden</span></div><p>visible</p>`
			require.Equal(t, "visible\n", htmltext.Convert(strings.NewReader(input)))
		})
	}
	// HTML allows these end tags to be omitted, and an end tag can appear
	// without a matching start tag; neither may unbalance the hidden state.
	for _, input := range []string{
		`<div style="display:none"><p>a<p>b</div><p>visible</p>`,
		`<div style="display:none"><ul><li>a<li>b</ul></div><p>visible</p>`,
		`<div style="display:none"><br></br>hidden</div><p>visible</p>`,
		`<div style="display:none"><span/>a</span>hidden</div><p>visible</p>`,
		`<span aria-hidden="TRUE">hidden</span><p>visible</p>`,
	} {
		require.Equal(t, "visible\n", htmltext.Convert(strings.NewReader(input)), input)
	}
	// command looks void but is an ordinary container, so its content is hidden.
	require.Equal(t, "visible\n", htmltext.Convert(strings.NewReader(
		`<command style="display:none">hidden</command><p>visible</p>`)))

	// Pins the boundary of the name stack: a hidden element that is never
	// closed explicitly hides the rest of the document.
	for _, input := range []string{
		`<td style="display:none">x<td>visible</td>`,
		`<div><span aria-hidden="true">x</div><p>visible</p>`,
	} {
		require.Equal(t, "\n", htmltext.Convert(strings.NewReader(input)), input)
	}
	for _, input := range []string{
		`<p>Before</p><img aria-hidden="true" src="x.png"><p>After</p>`,
		`<p>Before</p><div aria-hidden="true">x<br>y</div><p>After</p>`,
	} {
		require.Equal(t, "Before\nAfter\n", htmltext.Convert(strings.NewReader(input)))
	}
	for _, input := range []string{
		`<img style="display:none" alt="hidden"><p>visible</p>`,
		`<img style="display:none" alt="hidden"/><p>visible</p>`,
		`<div style="display:none"><img alt="hidden"/><br/><p>hidden</p></div><p>visible</p>`,
		`<div style="display:none"><style>p{color:red}</style><br>hidden</div><p>visible</p>`,
	} {
		require.Equal(t, "visible\n", htmltext.Convert(strings.NewReader(input)))
	}
}
