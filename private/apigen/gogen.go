// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
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

// MustWriteGo writes generated Go code into a file.
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
	var result string

	p := func(format string, a ...interface{}) {
		result += fmt.Sprintf(format+"\n", a...)
	}

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
			if getPackageName(path) == a.PackageName {
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

	for _, group := range a.EndpointGroups {
		for _, method := range group.endpoints {
			if method.Request != nil {
				i(getElementaryType(reflect.TypeOf(method.Request)).PkgPath())
			}
			if method.Response != nil {
				i(getElementaryType(reflect.TypeOf(method.Response)).PkgPath())
			}
		}
	}

	p("const dateLayout = \"2006-01-02T15:04:05.000Z\"")
	p("")

	for _, group := range a.EndpointGroups {
		i("github.com/zeebo/errs")
		p("var Err%sAPI = errs.Class(\"%s %s api\")", cases.Title(language.Und).String(group.Prefix), a.PackageName, group.Prefix)
	}

	p("")

	params := make(map[*fullEndpoint][]Param)

	for _, group := range a.EndpointGroups {
		p("type %sService interface {", group.Name)
		for _, e := range group.endpoints {
			params[e] = append(e.QueryParams, e.PathParams...)

			var paramStr string
			for _, param := range params[e] {
				paramStr += param.Type.String() + ", "
			}
			if e.Request != nil {
				paramStr += reflect.TypeOf(e.Request).String() + ", "
			}

			i("context", "storj.io/storj/private/api")
			if e.Response != nil {
				responseType := reflect.TypeOf(e.Response)
				returnParam := a.handleTypesPackage(responseType)
				if responseType == getElementaryType(responseType) {
					returnParam = "*" + returnParam
				}
				p("%s(context.Context, "+paramStr+") (%s, api.HTTPError)", e.MethodName, returnParam)
			} else {
				p("%s(context.Context, "+paramStr+") (api.HTTPError)", e.MethodName)
			}
		}
		p("}")
		p("")
	}

	for _, group := range a.EndpointGroups {
		i("go.uber.org/zap")
		p("// %sHandler is an api handler that exposes all %s related functionality.", group.Name, group.Prefix)
		p("type %sHandler struct {", group.Name)
		p("log *zap.Logger")
		p("service %sService", group.Name)
		p("auth api.Auth")
		p("}")
		p("")
	}

	for _, group := range a.EndpointGroups {
		i("github.com/gorilla/mux")
		p(
			"func New%s(log *zap.Logger, service %sService, router *mux.Router, auth api.Auth) *%sHandler {",
			group.Name,
			group.Name,
			group.Name,
		)
		p("handler := &%sHandler{", group.Name)
		p("log: log,")
		p("service: service,")
		p("auth: auth,")
		p("}")
		p("")
		p("%sRouter := router.PathPrefix(\"/api/v0/%s\").Subrouter()", group.Prefix, group.Prefix)
		for _, endpoint := range group.endpoints {
			handlerName := "handle" + endpoint.MethodName
			p("%sRouter.HandleFunc(\"%s\", handler.%s).Methods(\"%s\")", group.Prefix, endpoint.Path, handlerName, endpoint.Method)
		}
		p("")
		p("return handler")
		p("}")
		p("")
	}

	for _, group := range a.EndpointGroups {
		for _, endpoint := range group.endpoints {
			i("net/http")
			p("")
			handlerName := "handle" + endpoint.MethodName
			p("func (h *%sHandler) %s(w http.ResponseWriter, r *http.Request) {", group.Name, handlerName)
			p("ctx := r.Context()")
			p("var err error")
			p("defer mon.Task()(&ctx)(&err)")
			p("")

			p("w.Header().Set(\"Content-Type\", \"application/json\")")
			p("")

			if !endpoint.NoCookieAuth || !endpoint.NoAPIAuth {
				p("ctx, err = h.auth.IsAuthenticated(ctx, r, %v, %v)", !endpoint.NoCookieAuth, !endpoint.NoAPIAuth)
				p("if err != nil {")
				if !endpoint.NoCookieAuth {
					p("h.auth.RemoveAuthCookie(w)")
				}
				p("api.ServeError(h.log, w, http.StatusUnauthorized, err)")
				p("return")
				p("}")
				p("")
			}

			handleParams(p, i, endpoint.QueryParams, endpoint.PathParams)
			if endpoint.Request != nil {
				handleBody(p, endpoint.Request)
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
			p(methodFormat, endpoint.MethodName)
			p("if httpErr.Err != nil {")
			p("api.ServeError(h.log, w, httpErr.Status, httpErr.Err)")
			if endpoint.Response == nil {
				p("}")
				p("}")
				continue
			}
			p("return")
			p("}")

			i("encoding/json")
			p("")
			p("err = json.NewEncoder(w).Encode(retVal)")
			p("if err != nil {")
			p("h.log.Debug(\"failed to write json %s response\", zap.Error(Err%sAPI.Wrap(err)))", endpoint.MethodName, cases.Title(language.Und).String(group.Prefix))
			p("}")
			p("}")
		}
	}

	fileBody := result
	result = ""

	p("// AUTOGENERATED BY private/apigen")
	p("// DO NOT EDIT.")
	p("")

	p("package %s", a.PackageName)
	p("")

	p("import (")
	slices := [][]string{imports.Standard, imports.External, imports.Internal}
	for sn, slice := range slices {
		sort.Strings(slice)
		for pn, path := range slice {
			p(`"%s"`, path)
			if pn == len(slice)-1 && sn < len(slices)-1 {
				p("")
			}
		}
	}
	p(")")
	p("")

	result += fileBody

	output, err := format.Source([]byte(result))
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

// handleParams handles parsing of URL query parameters or path parameters.
func handleParams(p func(format string, a ...interface{}), i func(paths ...string), queryParams, pathParams []Param) {
	for _, params := range []*[]Param{&queryParams, &pathParams} {
		for _, param := range *params {
			varName := param.Name
			if param.Type != reflect.TypeOf("") {
				varName += "Param"
			}

			switch params {
			case &queryParams:
				p("%s := r.URL.Query().Get(\"%s\")", varName, param.Name)
				p("if %s == \"\" {", varName)
				p("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"parameter '%s' can't be empty\"))", param.Name)
				p("return")
				p("}")
				p("")
			case &pathParams:
				p("%s, ok := mux.Vars(r)[\"%s\"]", varName, param.Name)
				p("if !ok {")
				p("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"missing %s route param\"))", param.Name)
				p("return")
				p("}")
				p("")
			}

			switch param.Type {
			case reflect.TypeOf(uuid.UUID{}):
				i("storj.io/common/uuid")
				p("%s, err := uuid.FromString(%s)", param.Name, varName)
			case reflect.TypeOf(time.Time{}):
				i("time")
				p("%s, err := time.Parse(dateLayout, %s)", param.Name, varName)
			default:
				p("")
				continue
			}

			p("if err != nil {")
			p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
			p("return")
			p("}")
			p("")
		}
	}
}

// handleBody handles request body.
func handleBody(p func(format string, a ...interface{}), body interface{}) {
	p("payload := %s{}", reflect.TypeOf(body).String())
	p("if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {")
	p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	p("return")
	p("}")
	p("")
}

// getElementaryType simplifies a Go type.
func getElementaryType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
		return getElementaryType(t.Elem())
	default:
		return t
	}
}
