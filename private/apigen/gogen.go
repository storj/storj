// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"go/format"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"storj.io/common/uuid"
)

// DateFormat is the layout of dates passed into and out of the API.
const DateFormat = "2006-01-02T15:04:05.999Z"

// MustWriteGo writes generated Go code into a file.
// If an error occurs, it panics.
func (a *API) MustWriteGo(path string) {
	generated, err := a.generateGo()
	if err != nil {
		panic(errs.Wrap(err))
	}

	err = os.WriteFile(path, generated, 0644)
	if err != nil {
		panic(errs.Wrap(err))
	}
}

// generateGo generates api code and returns an output.
func (a *API) generateGo() ([]byte, error) {
	result := &StringBuilder{}
	pf := result.Writelnf

	getPackageName := func(path string) string {
		pathPackages := strings.Split(path, "/")
		return pathPackages[len(pathPackages)-1]
	}

	imports := struct {
		All      map[string]bool
		Standard []string
		External []string
		Internal []string
	}{
		All: make(map[string]bool),
	}

	i := func(paths ...string) {
		for _, path := range paths {
			if path == "" || getPackageName(path) == a.PackageName {
				continue
			}

			if _, ok := imports.All[path]; ok {
				continue
			}
			imports.All[path] = true

			var slice *[]string
			switch {
			case !strings.Contains(path, "."):
				slice = &imports.Standard
			case strings.HasPrefix(path, "storj.io"):
				slice = &imports.Internal
			default:
				slice = &imports.External
			}
			*slice = append(*slice, path)
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
		pf("var Err%sAPI = errs.Class(\"%s %s api\")", cases.Title(language.Und).String(group.Prefix), a.PackageName, group.Prefix)
	}

	pf("")

	params := make(map[*fullEndpoint][]Param)

	for _, group := range a.EndpointGroups {
		pf("type %sService interface {", group.Name)
		for _, e := range group.endpoints {
			params[e] = append(e.PathParams, e.QueryParams...)

			var paramStr string
			for i, param := range params[e] {
				paramStr += param.Name
				if i == len(params[e])-1 || param.Type != params[e][i+1].Type {
					paramStr += " " + param.Type.String()
				}
				paramStr += ", "
			}
			if e.Request != nil {
				paramStr += "request " + reflect.TypeOf(e.Request).String() + ", "
			}

			i("context", "storj.io/storj/private/api")
			if e.Response != nil {
				responseType := reflect.TypeOf(e.Response)
				returnParam := a.handleTypesPackage(responseType)
				if !isNillableType(responseType) {
					returnParam = "*" + returnParam
				}
				pf("%s(ctx context.Context, "+paramStr+") (%s, api.HTTPError)", e.MethodName, returnParam)
			} else {
				pf("%s(ctx context.Context, "+paramStr+") (api.HTTPError)", e.MethodName)
			}
		}
		pf("}")
		pf("")
	}

	for _, group := range a.EndpointGroups {
		i("go.uber.org/zap", "github.com/spacemonkeygo/monkit/v3")
		pf("// %sHandler is an api handler that exposes all %s related functionality.", group.Name, group.Prefix)
		pf("type %sHandler struct {", group.Name)
		pf("log *zap.Logger")
		pf("mon *monkit.Scope")
		pf("service %sService", group.Name)
		pf("auth api.Auth")
		pf("}")
		pf("")
	}

	for _, group := range a.EndpointGroups {
		i("github.com/gorilla/mux")
		pf(
			"func New%s(log *zap.Logger, mon *monkit.Scope, service %sService, router *mux.Router, auth api.Auth) *%sHandler {",
			group.Name,
			group.Name,
			group.Name,
		)
		pf("handler := &%sHandler{", group.Name)
		pf("log: log,")
		pf("mon: mon,")
		pf("service: service,")
		pf("auth: auth,")
		pf("}")
		pf("")
		pf("%sRouter := router.PathPrefix(\"/api/v0/%s\").Subrouter()", group.Prefix, group.Prefix)
		for _, endpoint := range group.endpoints {
			handlerName := "handle" + endpoint.MethodName
			pf("%sRouter.HandleFunc(\"%s\", handler.%s).Methods(\"%s\")", group.Prefix, endpoint.Path, handlerName, endpoint.Method)
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
			handlerName := "handle" + endpoint.MethodName
			pf("func (h *%sHandler) %s(w http.ResponseWriter, r *http.Request) {", group.Name, handlerName)
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
				handleBody(pf, endpoint.Request)
			}

			if !endpoint.NoCookieAuth || !endpoint.NoAPIAuth {
				pf("ctx, err = h.auth.IsAuthenticated(ctx, r, %v, %v)", !endpoint.NoCookieAuth, !endpoint.NoAPIAuth)
				pf("if err != nil {")
				if !endpoint.NoCookieAuth {
					pf("h.auth.RemoveAuthCookie(w)")
				}
				pf("api.ServeError(h.log, w, http.StatusUnauthorized, err)")
				pf("return")
				pf("}")
				pf("")
			}

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
			pf(methodFormat, endpoint.MethodName)
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
			pf("h.log.Debug(\"failed to write json %s response\", zap.Error(Err%sAPI.Wrap(err)))", endpoint.MethodName, cases.Title(language.Und).String(group.Prefix))
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

	pf("package %s", a.PackageName)
	pf("")

	pf("import (")
	slices := [][]string{imports.Standard, imports.External, imports.Internal}
	for sn, slice := range slices {
		sort.Strings(slice)
		for pn, path := range slice {
			pf(`"%s"`, path)
			if pn == len(slice)-1 && sn < len(slices)-1 {
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
		return nil, err
	}

	return output, nil
}

// handleTypesPackage handles the way some type is used in generated code.
// If type is from the same package then we use only type's name.
// If type is from external package then we use type along with its appropriate package name.
func (a *API) handleTypesPackage(t reflect.Type) string {
	if strings.HasPrefix(t.String(), a.PackageName) {
		return t.Elem().Name()
	}

	return t.String()
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

// handleBody handles request body.
func handleBody(pf func(format string, a ...interface{}), body interface{}) {
	pf("payload := %s{}", reflect.TypeOf(body).String())
	pf("if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {")
	pf("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	pf("return")
	pf("}")
	pf("")
}
