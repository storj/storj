// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package htmltext

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Convert extracts readable plain text from an HTML email template.
// Go template actions ({{ ... }}) are preserved as-is so the output is a valid text/template.
func Convert(r io.Reader) string {
	tok := html.NewTokenizer(r)

	var buf strings.Builder
	// skip: depth counter for title/style/script blocks whose content is never rendered.
	skip := 0
	// hidden: open tag names of an element not rendered to the reader, either
	// display:none (e.g. preheader divs) or aria-hidden (e.g. link separators).
	// Names rather than a depth, because HTML allows end tags to be omitted and
	// the tokenizer, unlike a parser, does not synthesize the missing ones.
	var hidden []string

	// linkHref is non-empty while we are inside an open <a href="...">.
	linkHref := ""
	var linkText strings.Builder

	write := func(s string) {
		if linkHref != "" {
			linkText.WriteString(s)
		} else {
			buf.WriteString(s)
		}
	}

	for {
		tt := tok.Next()
		if tt == html.ErrorToken {
			break
		}

		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			rawName, hasAttr := tok.TagName()
			tag := string(rawName)

			attrs := map[string]string{}
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = tok.TagAttr()
				attrs[string(k)] = string(v)
			}

			if tt == html.StartTagToken {
				switch tag {
				case "title", "style", "script":
					skip++
					continue
				}
			}
			if len(hidden) > 0 || strings.Contains(attrs["style"], "display:none") || strings.EqualFold(attrs["aria-hidden"], "true") {
				// The hidden element itself is assumed to be closed explicitly. Ending
				// it by an implied end tag, as in <td style="display:none">x<td>, would
				// need the open elements of the whole document, not just this subtree.
				// Void and self-closing elements have no end token to match.
				if tt == html.StartTagToken && !isVoidElement(tag) {
					hidden = append(hidden, tag)
				}
				continue
			}
			if skip > 0 {
				continue
			}

			switch tag {
			case "a":
				href := attrs["href"]
				if href != "" && href != "#" {
					linkHref = href
					linkText.Reset()
				}
			case "img":
				if alt := attrs["alt"]; alt != "" {
					write(alt)
				}
			case "br":
				write("\n")
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "div", "tr":
				write("\n")
			case "li":
				write("\n- ")
			}

		case html.EndTagToken:
			rawName, _ := tok.TagName()
			tag := string(rawName)

			switch tag {
			case "title", "style", "script":
				if skip > 0 {
					skip--
				}
				continue
			}
			if skip > 0 {
				continue
			}
			if len(hidden) > 0 {
				// Unwind to the matching name, dropping elements whose end tag was
				// omitted, and ignore an end tag that never opened.
				for i := len(hidden) - 1; i >= 0; i-- {
					if hidden[i] == tag {
						hidden = hidden[:i]
						break
					}
				}
				continue
			}

			switch tag {
			case "a":
				if linkHref != "" {
					text := strings.TrimSpace(linkText.String())
					if text != "" {
						buf.WriteString(text + " ( " + linkHref + " )")
					}
					linkHref = ""
				}
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "div", "tr":
				write("\n")
			}

		case html.TextToken:
			if skip == 0 && len(hidden) == 0 {
				write(string(tok.Text()))
			}
		default:
		}
	}

	lines := strings.Split(buf.String(), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// CollapseBlankLines reduces runs of blank lines to a single one and removes
// leading and trailing blank lines. Shared layout definitions render nothing,
// so the surrounding newlines would otherwise pad the plain-text message.
func CollapseBlankLines(s string) string {
	var out []string
	blank := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			blank = len(out) > 0
			continue
		}
		if blank {
			out = append(out, "")
		}
		blank = false
		out = append(out, line)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// isVoidElement reports whether an HTML element has no closing tag. It matches
// the elements golang.org/x/net/html treats as void, including the obsolete
// ones it still accepts.
func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "basefont", "bgsound", "br", "col", "embed", "frame",
		"hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}
