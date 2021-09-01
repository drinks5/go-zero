package gogen

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/tal-tech/go-zero/core/collection"
	"github.com/tal-tech/go-zero/tools/goctl/api/spec"
	"github.com/tal-tech/go-zero/tools/goctl/config"
	"github.com/tal-tech/go-zero/tools/goctl/util"
	"github.com/tal-tech/go-zero/tools/goctl/util/format"
	"github.com/tal-tech/go-zero/tools/goctl/vars"
)

const (
	routesFilename = "routes"
	routesTemplate = `// Code generated by goctl. DO NOT EDIT.
package handler

import (
	"net/http"

	{{.importPackages}}
)

func RegisterHandlers(engine *rest.Server, serverCtx *svc.ServiceContext) {
	engine.AddRoutes(
	{{range $group :=Groups}}
		{{range $route := $group.Routes}}
		[]rest.Route{
			{
				Method: .Method,
				Path: .Path,
				Handler: .Handler,
			}
		}
		{{end}}
		{{if .JwtEnabled}}
		rest.WithJwt(serverCtx.Config.{{group.AuthName}}.AccessSecret),
		{{end}}
		{{if .SignatureEnabled }}
		rest.WithSignature(serverCtx.Config.Signature),
		{{end}}
	{{end}}
	)
}
`
)

var mapping = map[string]string{
	"delete": "http.MethodDelete",
	"get":    "http.MethodGet",
	"head":   "http.MethodHead",
	"post":   "http.MethodPost",
	"put":    "http.MethodPut",
	"patch":  "http.MethodPatch",
}

type (
	group struct {
		Routes           []route
		JwtEnabled       bool
		SignatureEnabled bool
		AuthName         string
		Middlewares      []string
	}
	route struct {
		Method  string
		Path    string
		Handler string
	}
)

func genRoutes(dir, rootPkg string, cfg *config.Config, api *spec.ApiSpec) error {
	groups, err := getRoutes(api)
	if err != nil {
		return err
	}

	routeFilename, err := format.FileNamingFormat(cfg.NamingFormat, routesFilename)
	if err != nil {
		return err
	}
	routeFilename = routeFilename + ".go"

	filename := path.Join(dir, handlerDir, routeFilename)
	os.Remove(filename)
	data := struct {
		ImportPackages string
		Groups         []group
	}{
		ImportPackages: genRouteImports(rootPkg, api),
		Groups:         groups,
	}

	return genFile(fileGenConfig{
		dir:             dir,
		subdir:          handlerDir,
		filename:        routeFilename,
		templateName:    "routesTemplate",
		category:        category,
		templateFile:    routesTemplateFile,
		builtinTemplate: routesTemplate,
		data:            data,
	})
}

func genRouteImports(parentPkg string, api *spec.ApiSpec) string {
	importSet := collection.NewSet()
	importSet.AddStr(fmt.Sprintf("\"%s\"", util.JoinPackages(parentPkg, contextDir)))
	for _, group := range api.Service.Groups {
		for _, route := range group.Routes {
			folder := route.GetAnnotation(groupProperty)
			if len(folder) == 0 {
				folder = group.GetAnnotation(groupProperty)
				if len(folder) == 0 {
					continue
				}
			}
			importSet.AddStr(fmt.Sprintf("%s \"%s\"", toPrefix(folder), util.JoinPackages(parentPkg, handlerDir, folder)))
		}
	}
	imports := importSet.KeysStr()
	sort.Strings(imports)
	projectSection := strings.Join(imports, "\n\t")
	depSection := fmt.Sprintf("\"%s/rest\"", vars.ProjectOpenSourceURL)
	return fmt.Sprintf("%s\n\n\t%s", projectSection, depSection)
}

func getRoutes(api *spec.ApiSpec) ([]group, error) {
	var routes []group

	for _, g := range api.Service.Groups {
		var groupedRoutes group
		for _, r := range g.Routes {
			handler := getHandlerName(r)
			folder := r.GetAnnotation(groupProperty)
			if len(folder) > 0 {
				handler = toPrefix(folder) + "." + strings.ToUpper(handler[:1]) + handler[1:]
			} else {
				folder = g.GetAnnotation(groupProperty)
				if len(folder) > 0 {
					handler = toPrefix(folder) + "." + strings.ToUpper(handler[:1]) + handler[1:]
				}
			}
			groupedRoutes.Routes = append(groupedRoutes.Routes, route{
				Method:  r.Method,
				Path:    r.Path,
				Handler: handler,
			})
		}

		jwt := g.GetAnnotation("jwt")
		if len(jwt) > 0 {
			groupedRoutes.AuthName = jwt
			groupedRoutes.JwtEnabled = true
		}
		signature := g.GetAnnotation("signature")
		if signature == "true" {
			groupedRoutes.SignatureEnabled = true
		}
		middleware := g.GetAnnotation("middleware")
		if len(middleware) > 0 {
			for _, item := range strings.Split(middleware, ",") {
				groupedRoutes.Middlewares = append(groupedRoutes.Middlewares, item)
			}
		}
		routes = append(routes, groupedRoutes)
	}

	return routes, nil
}

func toPrefix(folder string) string {
	return strings.ReplaceAll(folder, "/", "")
}
