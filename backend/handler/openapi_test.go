package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPISpecMatchesRoutes(t *testing.T) {
	specPath, err := openAPISpecPath()
	if err != nil {
		t.Fatalf("resolve spec path: %v", err)
	}

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	pathsRaw, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("spec missing paths section")
	}

	specRoutes := make(map[string]map[string]bool)
	for path, methodsRaw := range pathsRaw {
		methodsObj, ok := methodsRaw.(map[string]any)
		if !ok {
			continue
		}
		if specRoutes[path] == nil {
			specRoutes[path] = make(map[string]bool)
		}
		for method := range methodsObj {
			specRoutes[path][strings.ToLower(method)] = true
		}
	}

	for _, route := range Routes {
		openapiPath := openAPIPath(route.Path)
		if _, ok := specRoutes[openapiPath]; !ok {
			t.Fatalf("spec missing path: %s", openapiPath)
		}
		method := strings.ToLower(route.Method)
		if !specRoutes[openapiPath][method] {
			t.Fatalf("spec missing method %s for path %s", method, openapiPath)
		}
	}

	for specPath, methods := range specRoutes {
		for method := range methods {
			if !routeExists(specPath, method) {
				t.Fatalf("route missing for spec path %s method %s", specPath, method)
			}
		}
	}
}

func openAPISpecPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "api", "openapi.json")), nil
}

func openAPIPath(ginPath string) string {
	parts := strings.Split(ginPath, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func routeExists(openapiPath, method string) bool {
	for _, route := range Routes {
		if strings.ToLower(route.Method) != method {
			continue
		}
		if openAPIPath(route.Path) == openapiPath {
			return true
		}
	}
	return false
}
