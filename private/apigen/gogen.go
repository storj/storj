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

	params := make(map[*FullEndpoint][]Param)

	for _, group := range a.EndpointGroups {
		// Define the service interface
		pf("type %sService interface {", capitalize(group.Name))
		for _, e := range group.endpoints {
			// Collect extra parameters from middleware.
			var extraParams []Param
			for _, m := range group.Middleware {
				extraParams = append(extraParams, m.ExtraServiceParams(a, group, e)...)
			}

			params[e] = append(extraParams, append(e.PathParams, e.QueryParams...)...)

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

		if len(middlewareArgs) > 0 {
			pf(
				"func New%s(log *zap.Logger, mon *monkit.Scope, service %sService, router *mux.Router, %s) *%sHandler {",
				cname,
				cname,
				strings.Join(middlewareArgs, ", "),
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
			pf("w.Header().Set(\"Content-Type\", \"application/json\")")
			pf("")

			if err := handleParams(result, i, endpoint.PathParams, endpoint.QueryParams); err != nil {
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
func handleParams(builder *StringBuilder, i func(paths ...string), pathParams, queryParams []Param) error {
	pf := builder.Writelnf
	pErrCheck := func() {
		pf("if err != nil {")
		pf("api.ServeError(h.log, w, http.StatusBadRequest, err)")
		pf("return")
		pf("}")
	}

	for _, params := range []*[]Param{&queryParams, &pathParams} {
		for _, param := range *params {
			varName := param.Name
			if param.Type.Kind() != reflect.String {
				varName += "Param"
			}

			switch params {
			case &queryParams:
				pf("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				pf("if %s == \"\" {", varName)
				pf("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"parameter '%s' can't be empty\"))", param.Name)
				pf("return")
				pf("}")
				pf("")
			case &pathParams:
				pf("%s, ok := mux.Vars(r)[\"%s\"]", varName, param.Name)
				pf("if !ok {")
				pf("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"missing %s route param\"))", param.Name)
				pf("return")
				pf("}")
				pf("")
			}

			switch param.Type {
			case reflect.TypeOf(uuid.UUID{}):
				i("storj.io/common/uuid")
				pf("%s, err := uuid.FromString(%s)", param.Name, varName)
				pErrCheck()
			case reflect.TypeOf(time.Time{}):
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
	}

	return nil
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
