// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: spanner-key-parser <key>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, `  spanner-key-parser 'objects(5\t3w\331>@7\224\0137^Q\277\226=,vivintclips30,\0026\347\343\226E\254wX\325\231\304J\221\016\252N\245\332=\334\034A\343g\014\233o%\235\036\\,\316y\335\202\312\346\275\257\245\370\256>\234/\002\247O\315@\353ln\316S\246\361\254\353O\017\347M_\027\344\036b\370\345\374\004\355\262\264ZX\326\362/\002I\345\"\261\333\356\341\374]>q\261N\235DV\335|R?-h\025\2311'\''o\311\331\265\303\215\\\\\304\225\227\344Z\266\234\327\252M\307\242_\312\310X\r\020\365\242\322g,-1905173148538669962)'`)
		fmt.Fprintln(os.Stderr, `  spanner-key-parser 'segments(\\)X\315x\227\001IG\236\224m\302\025o\204\221,-9223372036854775808)'`)
		os.Exit(1)
	}

	input := flag.Arg(0)
	if err := parseKey(input); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseKey(input string) error {
	// Determine the table type
	if strings.HasPrefix(input, "objects(") {
		return parseObjectsKey(input)
	} else if strings.HasPrefix(input, "segments(") {
		return parseSegmentsKey(input)
	}
	return fmt.Errorf("unknown key format: must start with 'objects(' or 'segments('")
}

func parseObjectsKey(input string) error {
	// Remove "objects(" prefix and ")" suffix
	content := strings.TrimPrefix(input, "objects(")
	content = strings.TrimSuffix(content, ")")

	// Parse the components - objects table has: project_id, bucket_name, object_key, version
	parts, err := parseComponents(content)
	if err != nil {
		return fmt.Errorf("failed to parse objects key: %w", err)
	}

	if len(parts) < 4 {
		return fmt.Errorf("expected at least 4 components for objects key, got %d", len(parts))
	}

	// The key format is: project_id, bucket_name, object_key, version
	// object_key may contain commas, so we take first, second, last, and join the rest
	projectID := parts[0]
	bucketName := parts[1]
	version := parts[len(parts)-1]

	// object_key is everything in between
	var objectKey []byte
	for i := 2; i < len(parts)-1; i++ {
		if i > 2 {
			objectKey = append(objectKey, ',')
		}
		objectKey = append(objectKey, parts[i]...)
	}

	fmt.Println("Table: objects")
	fmt.Printf("  project_id (hex): %s\n", hex.EncodeToString(projectID))
	fmt.Printf("  bucket_name: %s\n", formatPrintable(bucketName))
	fmt.Printf("  object_key (hex): %s\n", hex.EncodeToString(objectKey))
	fmt.Printf("  version: %s\n", formatPrintable(version))

	return nil
}

func parseSegmentsKey(input string) error {
	// Remove "segments(" prefix and ")" suffix
	content := strings.TrimPrefix(input, "segments(")
	content = strings.TrimSuffix(content, ")")

	// Parse the components - segments table has: stream_id, position
	parts, err := parseComponents(content)
	if err != nil {
		return fmt.Errorf("failed to parse segments key: %w", err)
	}

	if len(parts) != 2 {
		return fmt.Errorf("expected 2 components for segments key, got %d", len(parts))
	}

	fmt.Println("Table: segments")
	fmt.Printf("  stream_id: %s (hex: %s)\n", formatPrintable(parts[0]), hex.EncodeToString(parts[0]))

	// Try to parse position as int64
	posStr := string(parts[1])
	if pos, err := strconv.ParseInt(posStr, 10, 64); err == nil {
		fmt.Printf("  position: %d\n", pos)
	} else {
		fmt.Printf("  position: %s\n", formatPrintable(parts[1]))
	}

	return nil
}

// parseComponents splits the key content by commas, handling escaped sequences
func parseComponents(content string) ([][]byte, error) {
	var components [][]byte
	var current []byte

	i := 0
	for i < len(content) {
		if content[i] == ',' {
			// End of component
			components = append(components, current)
			current = nil
			i++
			continue
		}

		if content[i] == '\\' && i+1 < len(content) {
			// Escaped character
			next := content[i+1]
			switch next {
			case '\\':
				current = append(current, '\\')
				i += 2
			case 't':
				current = append(current, '\t')
				i += 2
			case 'n':
				current = append(current, '\n')
				i += 2
			case 'r':
				current = append(current, '\r')
				i += 2
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal escape sequence
				if i+3 < len(content) {
					octal := content[i+1 : i+4]
					val, err := strconv.ParseUint(octal, 8, 8)
					if err != nil {
						return nil, fmt.Errorf("invalid octal escape: \\%s", octal)
					}
					current = append(current, byte(val))
					i += 4
				} else {
					return nil, fmt.Errorf("incomplete octal escape at end of string")
				}
			default:
				// Unknown escape, treat as literal
				current = append(current, next)
				i += 2
			}
		} else {
			// Regular character
			current = append(current, content[i])
			i++
		}
	}

	// Don't forget the last component
	if current != nil {
		components = append(components, current)
	}

	return components, nil
}

// formatPrintable returns a string representation, showing hex for non-printable bytes
func formatPrintable(data []byte) string {
	if isAllPrintable(data) {
		return string(data)
	}

	// Mix of printable and non-printable - show both
	var result strings.Builder
	result.WriteString("\"")
	for _, b := range data {
		if b >= 32 && b < 127 && b != '"' && b != '\\' {
			result.WriteByte(b)
		} else {
			result.WriteString(fmt.Sprintf("\\x%02x", b))
		}
	}
	result.WriteString("\"")
	return result.String()
}

func isAllPrintable(data []byte) bool {
	for _, b := range data {
		r := rune(b)
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
