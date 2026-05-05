// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"go/format"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
)

// DateFormat is the layout of dates passed into and out of the API.
const DateFormat = "2006-01-02T15:04:05.999Z"

// MustWriteGo writes generated Go code into a file.
// If an error occurs, it panics.
func (a *API) MustWriteGo(path string) {
	generated, err := a.generateGo()
	if err != nil {
		panic(err)
	}

	rootDir := a.outputRootDir()
	fullpath := filepath.Join(rootDir, path)
	err = os.MkdirAll(filepath.Dir(fullpath), 0700)
	if err != nil {
		panic(errs.Wrap(err))
	}

	err = os.WriteFile(fullpath, generated, 0644)
	if err != nil {
		panic(errs.Wrap(err))
	}
}

// generateGo generates api code and returns an output.
func (a *API) generateGo() ([]byte, error) {
	result := &StringBuilder{}
	pf := result.Writelnf

	if a.PackagePath == "" {
		return nil, errs.New("Package path must be defined")
	}

	packageName := a.PackageName
	if packageName == "" {
		parts := strings.Split(a.PackagePath, "/")
		packageName = parts[len(parts)-1]
	}

	imports := struct {
		All      map[importPath]bool
		Standard []importPath
		External []importPath
		Internal []importPath
	}{
		All: make(map[importPath]bool),
	}

	i := func(paths ...string) {
		for _, path := range paths {
			if path == "" || path == a.PackagePath {
				continue
			}

			ipath := importPath(path)
			if _, ok := imports.All[ipath]; ok {
				continue
			}
			imports.All[ipath] = true

			var slice *[]importPath
			switch {
			case !strings.Contains(path, "."):
				slice = &imports.Standard
			case strings.HasPrefix(path, "storj.io"):
				slice = &imports.Internal
			default:
				slice = &imports.External
			}
			*slice = append(*slice, ipath)
		}
	}

	var getTypePackages func(t reflect.Type) []string
	getTypePackages = func(t reflect.Type) []string {
		t = getElementaryType(t)
		if t.Kind() == reflect.Map {
			pkgs := []string{getElementaryType(t.Key()).PkgPath()}
			return append(pkgs, getTypePackages(t.Elem())...)
		}
		return []string{t.PkgPath()}
	}

	for _, group := range a.EndpointGroups {
		for _, method := range group.endpoints {
			if method.Request != nil {
				i(getTypePackages(reflect.TypeOf(method.Request))...)
			}
			if method.Response != nil {
				i(getTypePackages(reflect.TypeOf(method.Response))...)
			}
		}
	}

	for _, group := range a.EndpointGroups {
		i("github.com/zeebo/errs")
		pf(
			"var Err%sAPI = errs.Class(\"%s %s api\")",
			capitalize(group.Prefix),
			packageName,
			strings.ToLower(group.Prefix),
		)

		for _, m := range group.Middleware {
			i(middlewareImports(m)...)
		}
	}

	pf("")

	params := make(map[*FullEndpoint][]PathParam)

	for _, group := range a.EndpointGroups {
		// Define the service interface
		pf("type %sService interface {", capitalize(group.Name))
		for _, e := range group.endpoints {
			// Collect extra parameters from middleware.
			var extraParams []PathParam
			for _, m := range group.Middleware {
				extraParams = append(extraParams, m.ExtraServiceParams(a, group, e)...)
			}

			params[e] = append(extraParams, e.PathParams...)
			for _, qp := range e.QueryParams {
				params[e] = append(params[e], qp.PathParam)
			}

			var paramStr string
			for i, param := range params[e] {
				paramStr += param.Name
				if i == len(params[e])-1 || param.Type != params[e][i+1].Type {
					if param.Type.Kind() == reflect.Pointer && param.Type.Elem().PkgPath() == a.PackagePath {
						paramStr += " *" + param.Type.Elem().Name()
					} else {
						paramStr += " " + param.Type.String()
					}
				}
				paramStr += ", "
			}
			if e.Request != nil {
				paramStr += "request " + a.handleTypesPackage(reflect.TypeOf(e.Request)) + ", "
			}

			i("context", "storj.io/storj/private/api")
			if e.Response != nil {
				responseType := reflect.TypeOf(e.Response)
				returnParam := a.handleTypesPackage(responseType)
				if !isNillableType(responseType) {
					returnParam = "*" + returnParam
				}
				pf("%s(ctx context.Context, "+paramStr+") (%s, api.HTTPError)", e.GoName, returnParam)
			} else if e.ResponseType != "" {
				i("net/http")
				pf("%s(ctx context.Context, w http.ResponseWriter, "+paramStr+") api.HTTPError", e.GoName)
			} else {
				pf("%s(ctx context.Context, "+paramStr+") (api.HTTPError)", e.GoName)
			}
		}
		pf("}")
		pf("")
	}

	for _, group := range a.EndpointGroups {
		cname := capitalize(group.Name)
		i("go.uber.org/zap", "github.com/spacemonkeygo/monkit/v3")
		pf(
			"// %sHandler is an api handler that implements all %s API endpoints functionality.",
			cname,
			group.Name,
		)
		pf("type %sHandler struct {", cname)
		pf("log *zap.Logger")
		pf("mon *monkit.Scope")
		pf("service %sService", cname)

		autodefinedFields := map[string]string{"log": "*zap.Logger", "mon": "*monkit.Scope", "service": cname + "Service"}
		for _, m := range group.Middleware {
			for _, f := range middlewareFields(a, m) {
				if t, ok := autodefinedFields[f.Name]; ok {
					if t != f.Type {
						panic(
							fmt.Sprintf(
								"middleware %q has a field with name %q and type %q which clashes with another defined field with the same name but with type %q",
								reflect.TypeOf(m).Name(),
								f.Name,
								f.Type,
								t,
							),
						)
					}

					continue
				}
				autodefinedFields[f.Name] = f.Type
				pf("%s %s", f.Name, f.Type)
			}
		}

		// Add a concrete typed field for each optional query param default value.
		// The field is always declared, even when the default is the type's zero
		// value, because handleParams references h.defaultXxx unconditionally at
		// request time. The constructor initialiser is suppressed for zero values
		// (see below) to avoid redundant assignments.
		for _, ep := range group.endpoints {
			for _, qp := range ep.QueryParams {
				if qp.Default != nil {
					fieldName := fmt.Sprintf("default%s%s", ep.GoName, capitalize(qp.PathParam.Name))
					typeName := defaultFieldTypeName(qp.PathParam.Type, i)
					if t, ok := autodefinedFields[fieldName]; ok {
						panic(
							fmt.Sprintf(
								"optional query param default field %q (for endpoint %q, param %q) clashes with an existing field of type %q",
								fieldName, ep.GoName, qp.PathParam.Name, t,
							),
						)
					}
					autodefinedFields[fieldName] = typeName
					pf("%s %s", fieldName, typeName)
				}
			}
		}

		// Add a func() interface{} field for each dynamic optional query param.
		// These are passed as constructor parameters and called at request time.
		for _, ep := range group.endpoints {
			for _, qp := range ep.QueryParams {
				if qp.DynamicDefault != nil {
					fieldName := fmt.Sprintf("default%s%s", ep.GoName, capitalize(qp.PathParam.Name))
					if t, ok := autodefinedFields[fieldName]; ok {
						panic(
							fmt.Sprintf(
								"dynamic query param default field %q (for endpoint %q, param %q) clashes with an existing field of type %q",
								fieldName, ep.GoName, qp.PathParam.Name, t,
							),
						)
					}
					autodefinedFields[fieldName] = "func() interface{}"
					pf("%s func() interface{}", fieldName)
				}
			}
		}

		pf("}")
		pf("")
	}

	for _, group := range a.EndpointGroups {
		cname := capitalize(group.Name)
		i("github.com/gorilla/mux")

		autodedefined := map[string]struct{}{"log": {}, "mon": {}, "service": {}}
		middlewareArgs := make([]string, 0, len(group.Middleware))
		middlewareFieldsList := make([]string, 0, len(group.Middleware))
		useCORS := false
		for _, m := range group.Middleware {
			if _, ok := m.(api.CORS); ok {
				useCORS = true
			}

			for _, f := range middlewareFields(a, m) {
				if _, ok := autodedefined[f.Name]; !ok {
					middlewareArgs = append(middlewareArgs, fmt.Sprintf("%s %s", f.Name, f.Type))
					middlewareFieldsList = append(middlewareFieldsList, fmt.Sprintf("%[1]s: %[1]s", f.Name))
				}
			}
		}

		// Collect dynamic default constructor params (one per dynamic optional param).
		dynamicDefaultArgs := make([]string, 0)
		dynamicDefaultInits := make([]string, 0)
		for _, ep := range group.endpoints {
			for _, qp := range ep.QueryParams {
				if qp.DynamicDefault != nil {
					fieldName := fmt.Sprintf("default%s%s", ep.GoName, capitalize(qp.PathParam.Name))
					dynamicDefaultArgs = append(dynamicDefaultArgs, fmt.Sprintf("%s func() interface{}", fieldName))
					dynamicDefaultInits = append(dynamicDefaultInits, fmt.Sprintf("%[1]s: %[1]s", fieldName))
				}
			}
		}

		//nolint:gocritic // suppress warning about not assigning to the same slice
		allExtraArgs := append(middlewareArgs, dynamicDefaultArgs...)
		if len(allExtraArgs) > 0 {
			pf(
				"func New%s(log *zap.Logger, mon *monkit.Scope, service %sService, router *mux.Router, %s) *%sHandler {",
				cname,
				cname,
				strings.Join(allExtraArgs, ", "),
				cname,
			)
		} else {
			pf(
				"func New%s(log *zap.Logger, mon *monkit.Scope, service %sService, router *mux.Router) *%sHandler {",
				cname,
				cname,
				cname,
			)
		}

		pf("handler := &%sHandler{", cname)
		pf("log: log,")
		pf("mon: mon,")
		pf("service: service,")

		if len(middlewareFieldsList) > 0 {
			pf(strings.Join(middlewareFieldsList, ",") + ",")
		}

		if len(dynamicDefaultInits) > 0 {
			pf(strings.Join(dynamicDefaultInits, ",") + ",")
		}

		// Initialize static optional query param default value fields.
		// Skip zero-value fields: the struct's zero-initialisation handles them.
		for _, ep := range group.endpoints {
			for _, qp := range ep.QueryParams {
				if qp.Default != nil && !isZeroDefault(qp.Default) {
					lit, litErr := defaultValueLiteral(qp, i)
					if litErr != nil {
						return nil, litErr
					}
					pf("default%s%s: %s,", ep.GoName, capitalize(qp.PathParam.Name), lit)
				}
			}
		}

		pf("}")
		pf("")
		pf(
			"%sRouter := router.PathPrefix(\"%s/%s\").Subrouter()",
			uncapitalize(group.Prefix),
			a.endpointBasePath(),
			strings.ToLower(group.Prefix),
		)
		for _, endpoint := range group.endpoints {
			handlerName := "handle" + endpoint.GoName

			methods := []string{endpoint.Method}
			if useCORS {
				methods = append(methods, http.MethodOptions)
			}

			pf(
				"%sRouter.HandleFunc(\"%s\", handler.%s).Methods(\"%s\")",
				uncapitalize(group.Prefix),
				endpoint.Path,
				handlerName,
				strings.Join(methods, ", "),
			)
		}
		pf("")
		pf("return handler")
		pf("}")
		pf("")
	}

	for _, group := range a.EndpointGroups {
		for _, endpoint := range group.endpoints {
			i("net/http")
			pf("")
			handlerName := "handle" + endpoint.GoName
			pf("func (h *%sHandler) %s(w http.ResponseWriter, r *http.Request) {", capitalize(group.Name), handlerName)
			pf("ctx := r.Context()")
			pf("var err error")
			pf("defer h.mon.Task()(&ctx)(&err)")
			pf("")
			if endpoint.ResponseType != "" {
				pf("w.Header().Set(\"Content-Type\", \"%s\")", endpoint.ResponseType)
			} else {
				pf("w.Header().Set(\"Content-Type\", \"application/json\")")
			}
			pf("")

			if err := handleParams(result, i, endpoint.GoName, endpoint.PathParams, endpoint.QueryParams); err != nil {
				return nil, err
			}

			if endpoint.Request != nil {
				a.handleBody(pf, endpoint.Request)
			}

			for _, m := range group.Middleware {
				pf(m.Generate(a, group, endpoint))
			}
			pf("")

			var methodFormat string
			if endpoint.Response != nil {
				methodFormat = "retVal, httpErr := h.service.%s(ctx, "
			} else if endpoint.ResponseType != "" {
				methodFormat = "httpErr := h.service.%s(ctx, w, "
			} else {
				methodFormat = "httpErr := h.service.%s(ctx, "
			}

			for _, param := range params[endpoint] {
				methodFormat += param.Name + ", "
			}
			if endpoint.Request != nil {
				methodFormat += "payload"
			}

			methodFormat += ")"
			pf(methodFormat, endpoint.GoName)
			pf("if httpErr.Err != nil {")
			pf("api.ServeError(h.log, w, httpErr.Status, httpErr.Err)")
			if endpoint.Response == nil {
				pf("}")
				pf("}")
				continue
			}
			pf("return")
			pf("}")

			i("encoding/json")
			pf("")
			pf("err = json.NewEncoder(w).Encode(retVal)")
			pf("if err != nil {")
			pf(
				"h.log.Debug(\"failed to write json %s response\", zap.Error(Err%sAPI.Wrap(err)))",
				endpoint.GoName,
				capitalize(group.Prefix),
			)
			pf("}")
			pf("}")
		}
	}

	fileBody := result.String()
	result = &StringBuilder{}
	pf = result.Writelnf

	pf("// AUTOGENERATED BY private/apigen")
	pf("// DO NOT EDIT.")
	pf("")

	pf("package %s", packageName)
	pf("")

	pf("import (")
	all := [][]importPath{imports.Standard, imports.External, imports.Internal}
	for sn, slice := range all {
		slices.Sort(slice)
		for pn, path := range slice {
			if r, ok := path.PkgName(); ok {
				pf(`%s "%s"`, r, path)
			} else {
				pf(`"%s"`, path)
			}

			if pn == len(slice)-1 && sn < len(all)-1 {
				pf("")
			}
		}
	}
	pf(")")
	pf("")

	if _, ok := imports.All["time"]; ok {
		pf("const dateLayout = \"%s\"", DateFormat)
		pf("")
	}

	result.WriteString(fileBody)

	output, err := format.Source([]byte(result.String()))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return output, nil
}

// handleTypesPackage handles the way some type is used in generated code.
// If type is from the same package then we use only type's name.
// If type is from external package then we use type along with its appropriate package name.
func (a *API) handleTypesPackage(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), a.handleTypesPackage(t.Elem()))
	case reflect.Slice:
		return "[]" + a.handleTypesPackage(t.Elem())
	case reflect.Pointer:
		return "*" + a.handleTypesPackage(t.Elem())
	}

	if t.PkgPath() == a.PackagePath {
		return t.Name()
	}

	return t.String()
}

// handleBody handles request body.
func (a *API) handleBody(pf func(format string, a ...interface{}), body interface{}) {
	pf("payload := %s{}", a.handleTypesPackage(reflect.TypeOf(body)))
	pf("if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {")
	pf("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	pf("return")
	pf("}")
	pf("")
}

// handleParams handles parsing of URL path parameters or query parameters.
// endpointGoName is the GoName of the endpoint, used to name handler struct fields
// for optional query param default functions.
func handleParams(builder *StringBuilder, i func(paths ...string), endpointGoName string, pathParams []PathParam, queryParams []QueryParam) error {
	pf := builder.Writelnf
	pErrCheck := func() {
		pf("if err != nil {")
		pf("api.ServeError(h.log, w, http.StatusBadRequest, err)")
		pf("return")
		pf("}")
	}
	pParseErrCheck := func() {
		pf("if parseErr != nil {")
		pf("api.ServeError(h.log, w, http.StatusBadRequest, parseErr)")
		pf("return")
		pf("}")
	}

	for _, qp := range queryParams {
		param := qp.PathParam
		varName := param.Name
		if param.Type.Kind() != reflect.String {
			varName += "Param"
		}

		switch {
		case qp.Default != nil:
			// Static optional param: use the handler's concrete default field value
			// when the key is absent from the URL.
			fieldName := fmt.Sprintf("h.default%s%s", endpointGoName, capitalize(param.Name))

			switch param.Type {
			case reflect.TypeFor[uuid.UUID]():
				i("storj.io/common/uuid")
				pf("var %s uuid.UUID", param.Name)
				pf("if r.URL.Query().Has(\"%s\") {", param.Name)
				pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				pf("var parseErr error")
				pf("%s, parseErr = uuid.FromString(%s)", param.Name, varName)
				pParseErrCheck()
				pf("} else {")
				pf("%s = %s", param.Name, fieldName)
				pf("}")
			case reflect.TypeFor[time.Time]():
				i("time")
				pf("var %s time.Time", param.Name)
				pf("if r.URL.Query().Has(\"%s\") {", param.Name)
				pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				pf("var parseErr error")
				pf("%s, parseErr = time.Parse(dateLayout, %s)", param.Name, varName)
				pParseErrCheck()
				pf("} else {")
				pf("%s = %s", param.Name, fieldName)
				pf("}")
			default:
				switch param.Type.Kind() {
				case reflect.String:
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("if !r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s = %s", varName, fieldName)
					pf("}")
				case reflect.Bool:
					i("strconv")
					pf("var %s bool", param.Name)
					pf("if r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("var parseErr error")
					pf("%s, parseErr = strconv.ParseBool(%s)", param.Name, varName)
					pParseErrCheck()
					pf("} else {")
					pf("%s = %s", param.Name, fieldName)
					pf("}")
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					i("strconv")
					bits := param.Type.Bits()
					if param.Type.Kind() == reflect.Uint {
						bits = 32
					}
					// Always use a distinct convName so that varName (the raw
					// string) and the parsed uint64 never share the same
					// identifier (which would prevent compilation for uint64).
					convName := param.Name + "U64"
					pf("var %s %s", param.Name, param.Type.String())
					pf("if r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("var %s uint64", convName)
					pf("var parseErr error")
					pf("%s, parseErr = strconv.ParseUint(%s, 10, %d)", convName, varName, bits)
					pParseErrCheck()
					if param.Type.Kind() == reflect.Uint64 {
						pf("%s = %s", param.Name, convName)
					} else {
						pf("%s = %s(%s)", param.Name, param.Type.String(), convName)
					}
					pf("} else {")
					pf("%s = %s", param.Name, fieldName)
					pf("}")
				default:
					return errs.New("Unsupported optional parameter type \"%s\"", param.Type)
				}
			}

		case qp.DynamicDefault != nil:
			// Dynamic optional param: call the handler's func() interface{} field
			// at request time when the key is absent from the URL. The function is
			// passed as a constructor parameter.
			fieldName := fmt.Sprintf("h.default%s%s", endpointGoName, capitalize(param.Name))

			switch param.Type {
			case reflect.TypeFor[uuid.UUID]():
				i("storj.io/common/uuid")
				pf("var %s uuid.UUID", param.Name)
				pf("if r.URL.Query().Has(\"%s\") {", param.Name)
				pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				pf("var parseErr error")
				pf("%s, parseErr = uuid.FromString(%s)", param.Name, varName)
				pParseErrCheck()
				pf("} else if v, ok := %s().(uuid.UUID); ok {", fieldName)
				pf("%s = v", param.Name)
				pf("}")
			case reflect.TypeFor[time.Time]():
				i("time")
				pf("var %s time.Time", param.Name)
				pf("if r.URL.Query().Has(\"%s\") {", param.Name)
				pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				pf("var parseErr error")
				pf("%s, parseErr = time.Parse(dateLayout, %s)", param.Name, varName)
				pParseErrCheck()
				pf("} else if v, ok := %s().(time.Time); ok {", fieldName)
				pf("%s = v", param.Name)
				pf("}")
			default:
				switch param.Type.Kind() {
				case reflect.String:
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("if !r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s, _ = %s().(string)", varName, fieldName)
					pf("}")
				case reflect.Bool:
					i("strconv")
					pf("var %s bool", param.Name)
					pf("if r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("var parseErr error")
					pf("%s, parseErr = strconv.ParseBool(%s)", param.Name, varName)
					pParseErrCheck()
					pf("} else if v, ok := %s().(bool); ok {", fieldName)
					pf("%s = v", param.Name)
					pf("}")
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					i("strconv")
					bits := param.Type.Bits()
					if param.Type.Kind() == reflect.Uint {
						bits = 32
					}
					convName := param.Name + "U64"
					pf("var %s %s", param.Name, param.Type.String())
					pf("if r.URL.Query().Has(\"%s\") {", param.Name)
					pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
					pf("var %s uint64", convName)
					pf("var parseErr error")
					pf("%s, parseErr = strconv.ParseUint(%s, 10, %d)", convName, varName, bits)
					pParseErrCheck()
					if param.Type.Kind() == reflect.Uint64 {
						pf("%s = %s", param.Name, convName)
					} else {
						pf("%s = %s(%s)", param.Name, param.Type.String(), convName)
					}
					pf("} else if v, ok := %s().(%s); ok {", fieldName, param.Type.String())
					pf("%s = v", param.Name)
					pf("}")
				default:
					return errs.New("Unsupported dynamic optional parameter type \"%s\"", param.Type)
				}
			}

		default:
			// Required param: get raw string, reject empty, then parse.
			pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
			pf("if %s == \"\" {", varName)
			pf("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"parameter '%s' can't be empty\"))", param.Name)
			pf("return")
			pf("}")
			pf("")

			switch param.Type {
			case reflect.TypeFor[uuid.UUID]():
				i("storj.io/common/uuid")
				pf("%s, err := uuid.FromString(%s)", param.Name, varName)
				pErrCheck()
			case reflect.TypeFor[time.Time]():
				i("time")
				pf("%s, err := time.Parse(dateLayout, %s)", param.Name, varName)
				pErrCheck()
			default:
				switch param.Type.Kind() {
				case reflect.String:
				case reflect.Bool:
					i("strconv")
					pf("%s, err := strconv.ParseBool(%s)", param.Name, varName)
					pErrCheck()
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					i("strconv")
					convName := varName
					if param.Type.Kind() != reflect.Uint64 {
						convName += "U64"
					}
					bits := param.Type.Bits()
					if param.Type.Kind() == reflect.Uint {
						bits = 32
					}
					pf("%s, err := strconv.ParseUint(%s, 10, %d)", convName, varName, bits)
					pErrCheck()
					if param.Type.Kind() != reflect.Uint64 {
						pf("%s := %s(%s)", param.Name, param.Type.String(), convName)
					}
				default:
					return errs.New("Unsupported parameter type \"%s\"", param.Type)
				}
			}
		}

		pf("")
	}

	for _, param := range pathParams {
		varName := param.Name
		if param.Type.Kind() != reflect.String {
			varName += "Param"
		}

		pf("%s, ok := mux.Vars(r)[\"%s\"]", varName, param.Name)
		pf("if !ok {")
		pf("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"missing %s route param\"))", param.Name)
		pf("return")
		pf("}")
		pf("")

		switch param.Type {
		case reflect.TypeFor[uuid.UUID]():
			i("storj.io/common/uuid")
			pf("%s, err := uuid.FromString(%s)", param.Name, varName)
			pErrCheck()
		case reflect.TypeFor[time.Time]():
			i("time")
			pf("%s, err := time.Parse(dateLayout, %s)", param.Name, varName)
			pErrCheck()
		default:
			switch param.Type.Kind() {
			case reflect.String:
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				i("strconv")
				convName := varName
				if param.Type.Kind() != reflect.Uint64 {
					convName += "U64"
				}
				bits := param.Type.Bits()
				if param.Type.Kind() == reflect.Uint {
					bits = 32
				}
				pf("%s, err := strconv.ParseUint(%s, 10, %d)", convName, varName, bits)
				pErrCheck()
				if param.Type.Kind() != reflect.Uint64 {
					pf("%s := %s(%s)", param.Name, param.Type.String(), convName)
				}
			default:
				return errs.New("Unsupported parameter type \"%s\"", param.Type)
			}
		}

		pf("")
	}

	return nil
}

// isZeroDefault reports whether v is the zero value of its type.
// It is used to suppress redundant zero-value field initialisers in generated
// constructor code.
func isZeroDefault(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.DeepEqual(v, reflect.Zero(reflect.TypeOf(v)).Interface())
}

// defaultValueLiteral returns the Go source literal for qp's default value and
// records any import paths needed by the literal via i.
//
// Supported default types: string (any value), time.Time (any value, converted
// to UTC), uuid.UUID (any value), and unsigned integer types (any value).
func defaultValueLiteral(qp QueryParam, i func(paths ...string)) (string, error) {
	v := qp.Default
	switch val := v.(type) {
	case bool:
		if val {
			return "true", nil
		}
		return "false", nil
	case string:
		return fmt.Sprintf("%q", val), nil
	case time.Time:
		if val.IsZero() {
			i("time")
			return "time.Time{}", nil
		}
		utc := val.UTC()
		i("time")
		return fmt.Sprintf("time.Date(%d, time.Month(%d), %d, %d, %d, %d, %d, time.UTC)",
			utc.Year(), int(utc.Month()), utc.Day(),
			utc.Hour(), utc.Minute(), utc.Second(), utc.Nanosecond()), nil
	case uuid.UUID:
		if val == (uuid.UUID{}) {
			i("storj.io/common/uuid")
			return "uuid.UUID{}", nil
		}
		i("storj.io/common/uuid")
		bytes := make([]string, 16)
		for j, b := range val {
			bytes[j] = fmt.Sprintf("0x%02x", b)
		}
		return fmt.Sprintf("uuid.UUID{%s}", strings.Join(bytes, ", ")), nil
	case uint:
		return fmt.Sprintf("uint(%d)", val), nil
	case uint8:
		return fmt.Sprintf("uint8(%d)", val), nil
	case uint16:
		return fmt.Sprintf("uint16(%d)", val), nil
	case uint32:
		return fmt.Sprintf("uint32(%d)", val), nil
	case uint64:
		return fmt.Sprintf("uint64(%d)", val), nil
	default:
		return "", errs.New("unsupported default value type %T for optional query param %q", v, qp.PathParam.Name)
	}
}

// defaultFieldTypeName returns the Go type name for the handler struct field
// that holds the default value of an optional query parameter. It also records
// any required import paths via i.
func defaultFieldTypeName(t reflect.Type, i func(paths ...string)) string {
	switch t {
	case reflect.TypeFor[uuid.UUID]():
		i("storj.io/common/uuid")
		return "uuid.UUID"
	case reflect.TypeFor[time.Time]():
		i("time")
		return "time.Time"
	default:
		// For non-builtin named types, register the import so the generated
		// code compiles. Built-in types (string, uint*, etc.) have an empty
		// PkgPath and t.String() is a valid unqualified identifier.
		if pkg := t.PkgPath(); pkg != "" {
			i(pkg)
		}
		return t.String()
	}
}

type importPath string

// PkgName returns the name of the package based of the last part of the import
// path and false if the name isn't a rename, otherwise it returns true.
//
// The package name is renamed when the last part of the path contains hyphen
// (-) or dot (.) and the rename is this part with the hyphens and dots
// stripped.
func (i importPath) PkgName() (rename string, ok bool) {
	b := filepath.Base(string(i))
	if strings.Contains(b, "-") || strings.Contains(b, ".") {
		return strings.ReplaceAll(strings.ReplaceAll(b, "-", ""), ".", ""), true
	}

	return b, false
}
