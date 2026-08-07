package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

var apiRouteLiteralPattern = regexp.MustCompile(`"(/api/[^"?]+)"`)
var restfulLiteralSegmentPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var restfulParameterSegmentPattern = regexp.MustCompile(`^[:*][a-z][a-z0-9_]*$`)

func TestAllAPIRouteDeclarationsFollowRESTConventions(t *testing.T) {
	root := ".."
	routeCalls := []string{"GinApi(", "Server.GET(", "Server.POST(", "Server.PUT(", "Server.PATCH(", "Server.DELETE(", "router.GET(", "router.POST(", "router.PUT(", "router.PATCH(", "router.DELETE("}
	unsupportedMethodMarkers := []string{"GinApi(PUT,", "GinApi(PATCH,", "GinApi(DELETE,", ".PUT(\"/api/", ".PATCH(\"/api/", ".DELETE(\"/api/"}
	forbiddenActionSegments := map[string]bool{
		"add": true, "decode": true, "delete": true, "download": true,
		"enable": true, "get": true, "list": true, "login": true,
		"outlogin": true, "register": true, "remove": true, "restart": true,
		"run": true, "set": true, "start": true, "update": true,
	}
	declared := 0
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
		if filepath.Ext(path) != ".go" {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			isRouteCall := false
			for _, marker := range routeCalls {
				if strings.Contains(line, marker) {
					isRouteCall = true
					break
				}
			}
			if !isRouteCall {
				continue
			}
			for _, marker := range unsupportedMethodMarkers {
				if strings.Contains(line, marker) {
					t.Errorf("%s:%d API declarations may only use GET or POST: %s", path, lineNumber+1, strings.TrimSpace(line))
				}
			}
			match := apiRouteLiteralPattern.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}
			declared++
			route := match[1]
			for _, segment := range strings.Split(strings.Trim(route, "/"), "/") {
				if restfulParameterSegmentPattern.MatchString(segment) {
					continue
				}
				if !restfulLiteralSegmentPattern.MatchString(segment) {
					t.Errorf("%s:%d route %q has a non-REST path segment %q", path, lineNumber+1, route, segment)
				}
				if forbiddenActionSegments[segment] {
					t.Errorf("%s:%d route %q exposes action segment %q instead of a resource", path, lineNumber+1, route, segment)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if declared < 100 {
		t.Fatalf("route audit covered only %d declarations; expected at least 100", declared)
	}
}

func TestAPIRouteSnapshotSupportsConcurrentRegistration(t *testing.T) {
	ssMutex.Lock()
	original := append([]Req(nil), ss...)
	ssMutex.Unlock()
	defer func() {
		ssMutex.Lock()
		ss = original
		ssMutex.Unlock()
	}()

	const writers = 16
	var wait sync.WaitGroup
	wait.Add(writers * 2)
	for index := 0; index < writers; index++ {
		go func(index int) {
			defer wait.Done()
			GinApi(GET, fmt.Sprintf("/api/concurrency-probes/%d", index), func(*gin.Context) {})
		}(index)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 100; iteration++ {
				_ = apiRouteSnapshot()
			}
		}()
	}
	wait.Wait()
	if got := len(apiRouteSnapshot()); got != len(original)+writers {
		t.Fatalf("registered route count=%d; want=%d", got, len(original)+writers)
	}
}

func TestRegisteredRoutesHaveNoMethodPathDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, route := range apiRouteSnapshot() {
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Errorf("duplicate route registration: %s", key)
		}
		seen[key] = true
	}
}

func TestRegisteredAPIRoutesUseOnlyGETAndPOST(t *testing.T) {
	for _, route := range apiRouteSnapshot() {
		if route.Method != GET && route.Method != POST {
			t.Errorf("API route uses unsupported method %s: %s", route.Method, route.Path)
		}
	}
}
