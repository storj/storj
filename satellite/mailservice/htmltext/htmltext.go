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
	skip := 0 // depth counter: >0 means we are inside head/style/script.

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

			// track nesting depth for skipped blocks even when already inside one.
			if tt == html.StartTagToken {
				switch tag {
				case "head", "style", "script":
					skip++
					continue
				}
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
			case "head", "style", "script":
				if skip > 0 {
					skip--
				}
				continue
			}
			if skip > 0 {
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
			if skip == 0 {
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
