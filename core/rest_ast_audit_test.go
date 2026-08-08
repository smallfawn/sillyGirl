package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type sourceAPIRoute struct {
	Method string
	Path   string
	File   string
	Line   int
}

func goSourceFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "admin" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func callName(call *ast.CallExpr) string {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	default:
		return ""
	}
}

func stringValue(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func httpMethodValue(expression ast.Expr) string {
	name := ""
	switch value := expression.(type) {
	case *ast.Ident:
		name = value.Name
	case *ast.SelectorExpr:
		name = value.Sel.Name
	case *ast.BasicLit:
		if method, ok := stringValue(value); ok {
			return strings.ToUpper(method)
		}
	}
	switch name {
	case "GET", "MethodGet":
		return "GET"
	case "POST", "MethodPost":
		return "POST"
	case "PUT", "MethodPut":
		return "PUT"
	case "PATCH", "MethodPatch":
		return "PATCH"
	case "DELETE", "MethodDelete":
		return "DELETE"
	case "HEAD", "MethodHead":
		return "HEAD"
	case "OPTIONS", "MethodOptions":
		return "OPTIONS"
	case "ANY", "Any":
		return "ANY"
	default:
		return strings.ToUpper(name)
	}
}

func routeFromCall(call *ast.CallExpr) (string, string, bool) {
	name := callName(call)
	methodIndex, pathIndex := -1, -1
	switch name {
	case "GinApi":
		methodIndex, pathIndex = 0, 1
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "Any", "ANY":
		pathIndex = 0
	case "Handle":
		methodIndex, pathIndex = 0, 1
	case "HandleFunc":
		pathIndex = 0
	default:
		return "", "", false
	}
	if pathIndex >= len(call.Args) {
		return "", "", false
	}
	path, ok := stringValue(call.Args[pathIndex])
	if !ok || (path != "/api" && !strings.HasPrefix(path, "/api/")) {
		return "", "", false
	}
	method := "ANY"
	if methodIndex >= 0 {
		if methodIndex >= len(call.Args) {
			return "", "", false
		}
		method = httpMethodValue(call.Args[methodIndex])
	} else if name != "HandleFunc" {
		method = httpMethodValue(&ast.Ident{Name: name})
	}
	return method, path, true
}

func productionAPIRoutes(root string) ([]sourceAPIRoute, error) {
	files, err := goSourceFiles(root)
	if err != nil {
		return nil, err
	}
	fileSet := token.NewFileSet()
	routes := []sourceAPIRoute{}
	for _, path := range files {
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, route, ok := routeFromCall(call)
			if !ok {
				return true
			}
			position := fileSet.Position(call.Pos())
			routes = append(routes, sourceAPIRoute{Method: method, Path: route, File: position.Filename, Line: position.Line})
			return true
		})
	}
	return routes, nil
}

func TestProductionAPIDeclarationsUseGETPOSTResourcePaths(t *testing.T) {
	routes, err := productionAPIRoutes("..")
	if err != nil {
		t.Fatal(err)
	}
	forbiddenActionSegments := map[string]bool{
		"add": true, "decode": true, "delete": true, "download": true,
		"enable": true, "get": true, "list": true, "login": true,
		"outlogin": true, "register": true, "remove": true, "restart": true,
		"run": true, "set": true, "start": true, "update": true,
	}
	for _, route := range routes {
		if route.Method != GET && route.Method != POST {
			t.Errorf("%s:%d API route uses %s; only GET and POST are allowed: %s", route.File, route.Line, route.Method, route.Path)
		}
		for _, segment := range strings.Split(strings.Trim(route.Path, "/"), "/") {
			if restfulParameterSegmentPattern.MatchString(segment) {
				continue
			}
			if !restfulLiteralSegmentPattern.MatchString(segment) {
				t.Errorf("%s:%d route %q has a non-resource path segment %q", route.File, route.Line, route.Path, segment)
			}
			if forbiddenActionSegments[segment] {
				t.Errorf("%s:%d route %q exposes action segment %q", route.File, route.Line, route.Path, segment)
			}
		}
	}
	if len(routes) < 105 {
		t.Fatalf("AST route audit covered only %d declarations; expected at least 105", len(routes))
	}
	t.Logf("audited %d production API route declarations", len(routes))
}

func TestRemovedUnusedAPIRoutesStayRemoved(t *testing.T) {
	routes, err := productionAPIRoutes("..")
	if err != nil {
		t.Fatal(err)
	}
	removed := map[string]bool{
		"/api/admin/carry-group-names": true,
		"/api/admin/carry-scripts":     true,
		"/api/admin/carry-rules":       true,
		"/api/admin/nickname-labels":   true,
	}
	for _, route := range routes {
		if removed[route.Path] {
			t.Errorf("removed API route was registered again: %s %s (%s:%d)", route.Method, route.Path, route.File, route.Line)
		}
	}
}

func expressionContainsIdentifier(expression ast.Expr, name string) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func TestAdminAPIRoutesRequireAuthentication(t *testing.T) {
	public := map[string]bool{
		"GET /api/admin/setup":     true,
		"POST /api/admin/setup":    true,
		"POST /api/admin/sessions": true,
	}
	files, err := goSourceFiles("..")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	checked := 0
	for _, path := range files {
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, route, ok := routeFromCall(call)
			if !ok || !strings.HasPrefix(route, "/api/admin/") {
				return true
			}
			checked++
			key := method + " " + route
			if public[key] {
				return true
			}
			if callName(call) != "GinApi" {
				position := fileSet.Position(call.Pos())
				t.Errorf("%s:%d protected admin route bypasses GinApi middleware: %s", position.Filename, position.Line, key)
				return true
			}
			hasAuth := false
			for _, argument := range call.Args[2:] {
				if expressionContainsIdentifier(argument, "RequireAuth") {
					hasAuth = true
					break
				}
			}
			if !hasAuth {
				position := fileSet.Position(call.Pos())
				t.Errorf("%s:%d admin route lacks RequireAuth: %s", position.Filename, position.Line, key)
			}
			return true
		})
	}
	if checked < 80 {
		t.Fatalf("admin auth audit covered only %d routes", checked)
	}
}

func TestUserAPIRoutesRequireAuthentication(t *testing.T) {
	public := map[string]bool{
		"POST /api/user/accounts": true,
		"POST /api/user/sessions": true,
	}
	files, err := goSourceFiles("..")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	checked := 0
	for _, path := range files {
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, route, ok := routeFromCall(call)
			if !ok || !strings.HasPrefix(route, "/api/user/") {
				return true
			}
			checked++
			key := method + " " + route
			if public[key] {
				return true
			}
			position := fileSet.Position(call.Pos())
			if callName(call) != "GinApi" {
				t.Errorf("%s:%d protected user route bypasses GinApi middleware: %s", position.Filename, position.Line, key)
				return true
			}
			hasAuth := false
			for _, argument := range call.Args[2:] {
				if expressionContainsIdentifier(argument, "RequireUserAuth") {
					hasAuth = true
					break
				}
			}
			if !hasAuth {
				t.Errorf("%s:%d user route lacks RequireUserAuth: %s", position.Filename, position.Line, key)
			}
			return true
		})
	}
	if checked < 18 {
		t.Fatalf("user auth audit covered only %d routes", checked)
	}
}

func internalAPIPath(expression ast.Expr) string {
	path := ""
	ast.Inspect(expression, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil {
			if index := strings.Index(value, "/api/"); index >= 0 {
				path = value[index:]
				return false
			}
		}
		return true
	})
	return path
}

func TestProductionAPIClientsUseOnlyGETAndPOST(t *testing.T) {
	files, err := goSourceFiles("..")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	checked := 0
	for _, path := range files {
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			methodIndex, urlIndex := -1, -1
			switch callName(call) {
			case "NewBeegoRequest":
				urlIndex, methodIndex = 0, 1
			case "NewRequest":
				methodIndex, urlIndex = 0, 1
			case "NewRequestWithContext":
				methodIndex, urlIndex = 1, 2
			default:
				return true
			}
			if methodIndex >= len(call.Args) || urlIndex >= len(call.Args) {
				return true
			}
			apiPath := internalAPIPath(call.Args[urlIndex])
			if apiPath == "" {
				return true
			}
			checked++
			method := httpMethodValue(call.Args[methodIndex])
			if method != GET && method != POST {
				position := fileSet.Position(call.Pos())
				t.Errorf("%s:%d internal API client uses %s for %s", position.Filename, position.Line, method, apiPath)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("AST client audit did not inspect any internal API request")
	}
	t.Logf("audited %d production API client requests", checked)
}
