// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/uuid"
)

// MustWriteDocs generates API documentation and writes it to the specified file path.
// If an error occurs, it panics.
func (api *API) MustWriteDocs(path string) {
	docs := api.generateDocumentation()

	rootDir := api.outputRootDir()
	fullpath := filepath.Join(rootDir, path)
	err := os.MkdirAll(filepath.Dir(fullpath), 0700)
	if err != nil {
		panic(errs.Wrap(err))
	}

	err = os.WriteFile(fullpath, []byte(docs), 0644)
	if err != nil {
		panic(errs.Wrap(err))
	}
}

// generateDocumentation generates a string containing the API documentation.
func (api *API) generateDocumentation() string {
	var doc strings.Builder

	wf := func(format string, args ...any) { _, _ = fmt.Fprintf(&doc, format, args...) }

	wf("# API Docs\n\n")
	if api.Description != "" {
		wf("**Description:** %s\n\n", api.Description)
	}

	if api.Version != "" {
		wf("**Version:** `%s`\n\n", api.Version)
	}

	wf("<h2 id='list-of-endpoints'>List of Endpoints</h2>\n\n")
	getEndpointLink := func(group, endpoint string) string {
		fullName := group + "-" + endpoint
		fullName = strings.ReplaceAll(fullName, " ", "-")
		nonAlphanumericRegex := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
		fullName = nonAlphanumericRegex.ReplaceAllString(fullName, "")
		return strings.ToLower(fullName)
	}
	for _, group := range api.EndpointGroups {
		wf("* %s\n", group.Name)
		for _, endpoint := range group.endpoints {
			wf("  * [%s](#%s)\n", endpoint.Name, getEndpointLink(group.Name, endpoint.Name))
		}
	}
	wf("\n")

	for _, group := range api.EndpointGroups {
		for _, endpoint := range group.endpoints {
			wf(
				"<h3 id='%s'>%s (<a href='#list-of-endpoints'>go to full list</a>)</h3>\n\n",
				getEndpointLink(group.Name, endpoint.Name),
				endpoint.Name,
			)
			wf("%s\n\n", endpoint.Description)
			wf("`%s %s/%s%s`\n\n", endpoint.Method, api.endpointBasePath(), group.Prefix, endpoint.Path)

			if len(endpoint.QueryParams) > 0 {
				wf("**Query Params:**\n\n")
				wf("| name | type | elaboration |\n")
				wf("|---|---|---|\n")
				for _, param := range endpoint.QueryParams {
					typeStr, elaboration := getDocType(param.Type)
					wf("| `%s` | `%s` | %s |\n", param.Name, typeStr, elaboration)
				}
				wf("\n")
			}
			if len(endpoint.PathParams) > 0 {
				wf("**Path Params:**\n\n")
				wf("| name | type | elaboration |\n")
				wf("|---|---|---|\n")
				for _, param := range endpoint.PathParams {
					typeStr, elaboration := getDocType(param.Type)
					wf("| `%s` | `%s` | %s |\n", param.Name, typeStr, elaboration)
				}
				wf("\n")
			}
			requestType := reflect.TypeOf(endpoint.Request)
			if requestType != nil {
				wf("**Request body:**\n\n")
				wf("```typescript\n%s\n```\n\n", getTypeNameRecursively(requestType, 0))
			}

			responseType := reflect.TypeOf(endpoint.Response)
			if responseType != nil {
				wf("**Response body:**\n\n")
				wf("```typescript\n%s\n```\n\n", getTypeNameRecursively(responseType, 0))
			}
		}
	}

	return doc.String()
}

// getDocType returns the "basic" type to use in JSON, as well as examples for specific types that may require elaboration.
func getDocType(t reflect.Type) (typeStr, elaboration string) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t {
	case reflect.TypeOf(uuid.UUID{}):
		return "string", "UUID formatted as `00000000-0000-0000-0000-000000000000`"
	case reflect.TypeOf(time.Time{}):
		return "string", "Date timestamp formatted as `2006-01-02T15:00:00Z`"
	case reflect.TypeOf(memory.Size(0)):
		return "string", "Amount of memory formatted as `15 GB`"
	default:
		switch t.Kind() {
		case reflect.String:
			return "string", ""
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "number", ""
		case reflect.Float32, reflect.Float64:
			return "number", ""
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "number", ""
		case reflect.Bool:
			return "boolean", ""
		}
	}

	return "unknown", ""
}

// getTypeNameRecursively gets a "full type" to document a struct or slice, including proper indentation of child properties.
func getTypeNameRecursively(t reflect.Type, level int) string {
	prefix := ""
	for i := 0; i < level; i++ {
		prefix += "\t"
	}

	switch t.Kind() {
	case reflect.Slice:
		elemType := t.Elem()
		if elemType.Kind() == reflect.Uint8 { // treat []byte as string in docs
			return prefix + "string"
		}
		return fmt.Sprintf("%s[\n%s\n%s]\n", prefix, getTypeNameRecursively(elemType, level+1), prefix)
	case reflect.Struct:
		// some struct types may actually be documented as elementary types; check first
		typeName, elaboration := getDocType(t)
		if typeName != "unknown" {
			toReturn := typeName
			if len(elaboration) > 0 {
				toReturn += " // " + elaboration
			}
			return toReturn
		}

		cfields := GetClassFieldsFromStruct(t)
		fields := make([]string, 0, len(cfields))
		for _, f := range cfields {
			fields = append(fields, prefix+"\t"+f.Name+": "+getTypeNameRecursively(f.Type, level+1))
		}

		return fmt.Sprintf("%s{\n%s\n%s}\n", prefix, strings.Join(fields, "\n"), prefix)
	default:
		typeName, elaboration := getDocType(t)
		toReturn := typeName
		if len(elaboration) > 0 {
			toReturn += " // " + elaboration
		}
		return toReturn
	}
}
