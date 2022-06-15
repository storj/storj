// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"go/format"
	"net/http"
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
				return
			}

			if _, ok := imports.All[path]; ok {
				return
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
		for _, method := range group.Endpoints {
			if method.Request != nil {
				i(reflect.TypeOf(method.Request).Elem().PkgPath())
			}
			if method.Response != nil {
				i(reflect.TypeOf(method.Response).Elem().PkgPath())
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

	for _, group := range a.EndpointGroups {
		p("type %sService interface {", group.Name)
		for _, e := range group.Endpoints {
			var params string
			for _, param := range e.Params {
				params += param.Type.String() + ", "
			}

			i("context", "storj.io/storj/private/api")
			if e.Response != nil {
				responseType := reflect.TypeOf(e.Response)
				p("%s(context.Context, "+params+") (%s, api.HTTPError)", e.MethodName, a.handleTypesPackage(responseType))
			} else {
				p("%s(context.Context, "+params+") (api.HTTPError)", e.MethodName)
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
		for pathMethod, endpoint := range group.Endpoints {
			handlerName := "handle" + endpoint.MethodName
			p("%sRouter.HandleFunc(\"%s\", handler.%s).Methods(\"%s\")", group.Prefix, pathMethod.Path, handlerName, pathMethod.Method)
		}
		p("")
		p("return handler")
		p("}")
		p("")
	}

	for _, group := range a.EndpointGroups {
		for pathMethod, endpoint := range group.Endpoints {
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

			switch pathMethod.Method {
			case http.MethodGet:
				for _, param := range endpoint.Params {
					switch param.Type {
					case reflect.TypeOf(uuid.UUID{}):
						i("storj.io/common/uuid")
						handleUUIDQuery(p, param)
						continue
					case reflect.TypeOf(time.Time{}):
						i("time")
						handleTimeQuery(p, param)
						continue
					case reflect.TypeOf(""):
						handleStringQuery(p, param)
						continue
					}
				}
			case http.MethodPatch:
				for _, param := range endpoint.Params {
					if param.Type == reflect.TypeOf(uuid.UUID{}) {
						handleUUIDParam(p, param)
					} else {
						handleBody(p, param)
					}
				}
			case http.MethodPost:
				for _, param := range endpoint.Params {
					handleBody(p, param)
				}
			case http.MethodDelete:
				for _, param := range endpoint.Params {
					handleUUIDParam(p, param)
				}
			}

			var methodFormat string
			if endpoint.Response != nil {
				methodFormat = "retVal, httpErr := h.service.%s(ctx, "
			} else {
				methodFormat = "httpErr := h.service.%s(ctx, "
			}

			switch pathMethod.Method {
			case http.MethodGet:
				for _, methodParam := range endpoint.Params {
					methodFormat += methodParam.Name + ", "
				}
			case http.MethodPatch:
				for _, methodParam := range endpoint.Params {
					if methodParam.Type == reflect.TypeOf(uuid.UUID{}) {
						methodFormat += methodParam.Name + ", "
					} else {
						methodFormat += "*" + methodParam.Name + ", "
					}
				}
			case http.MethodPost:
				for _, methodParam := range endpoint.Params {
					methodFormat += "*" + methodParam.Name + ", "
				}
			case http.MethodDelete:
				for _, methodParam := range endpoint.Params {
					methodFormat += methodParam.Name + ", "
				}
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
func (a *API) handleTypesPackage(t reflect.Type) interface{} {
	if strings.HasPrefix(t.String(), a.PackageName) {
		return t.Elem().Name()
	}

	return t
}

// handleStringQuery handles request query param of type string.
func handleStringQuery(p func(format string, a ...interface{}), param Param) {
	p("%s := r.URL.Query().Get(\"%s\")", param.Name, param.Name)
	p("if %s == \"\" {", param.Name)
	p("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"parameter '%s' can't be empty\"))", param.Name)
	p("return")
	p("}")
	p("")
}

// handleUUIDQuery handles request query param of type uuid.UUID.
func handleUUIDQuery(p func(format string, a ...interface{}), param Param) {
	p("%s, err := uuid.FromString(r.URL.Query().Get(\"%s\"))", param.Name, param.Name)
	p("if err != nil {")
	p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	p("return")
	p("}")
	p("")
}

// handleTimeQuery handles request query param of type time.Time.
func handleTimeQuery(p func(format string, a ...interface{}), param Param) {
	p("%s, err := time.Parse(dateLayout, r.URL.Query().Get(\"%s\"))", param.Name, param.Name)
	p("if err != nil {")
	p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	p("return")
	p("}")
	p("")
}

// handleUUIDParam handles request inline param of type uuid.UUID.
func handleUUIDParam(p func(format string, a ...interface{}), param Param) {
	p("%sParam, ok := mux.Vars(r)[\"%s\"]", param.Name, param.Name)
	p("if !ok {")
	p("api.ServeError(h.log, w, http.StatusBadRequest, errs.New(\"missing %s route param\"))", param.Name)
	p("return")
	p("}")
	p("")

	p("%s, err := uuid.FromString(%sParam)", param.Name, param.Name)
	p("if err != nil {")
	p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	p("return")
	p("}")
	p("")
}

// handleBody handles request body.
func handleBody(p func(format string, a ...interface{}), param Param) {
	p("%s := &%s{}", param.Name, param.Type)
	p("if err = json.NewDecoder(r.Body).Decode(&%s); err != nil {", param.Name)
	p("api.ServeError(h.log, w, http.StatusBadRequest, err)")
	p("return")
	p("}")
	p("")
}
