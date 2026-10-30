package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvaldes14/decision-gateway/internal/engine"
	"github.com/mvaldes14/decision-gateway/internal/provider"
	"github.com/mvaldes14/decision-gateway/internal/registry"
)

const testDefinition = `apiVersion: decisiongateway.dev/v1alpha1
kind: Decision
metadata: {name: test, version: v1}
spec:
  description: test
  input:
    type: object
    additionalProperties: false
    required: [message]
    properties:
      message: {type: string, minLength: 1}
  decisions:
    label:
      type: choice
      instruction: Pick one.
      options: {yes: Positive, no: Negative}
  routing: {primary: missing, timeout: 1s}
  review: {enabled: false}
`

func TestEndpoints(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "test.yaml"), []byte(testDefinition), 0o600); err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(definitions, engine.New(provider.NewSet()), 1024, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{http.MethodGet, "/healthz", "", http.StatusOK, `"status":"ok"`},
		{http.MethodGet, "/readyz", "", http.StatusServiceUnavailable, `"NOT_READY"`},
		{http.MethodGet, "/v1/decisions", "", http.StatusOK, `"name":"test"`},
		{http.MethodGet, "/v1/decisions/test/versions/v1", "", http.StatusOK, `"input_schema"`},
		{http.MethodPost, "/v1/decisions/test/versions/v1:execute", `{}`, http.StatusUnprocessableEntity, `"INVALID_INPUT"`},
		{http.MethodPost, "/v1/decisions/test/versions/v1:execute", `{"message":"hello"}`, http.StatusBadGateway, `"PROVIDER_ERROR"`},
	} {
		t.Run(test.method+test.path+test.body, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing X-Request-ID")
			}
		})
	}
}

func TestExecuteWithSystemOneProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			t.Fatalf("upstream path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answers":{"label":{"type":"choice","choice":"yes","probabilities":{"yes":0.95,"no":0.05}}}}`))
	}))
	defer upstream.Close()

	directory := t.TempDir()
	definition := strings.Replace(testDefinition, "primary: missing", "primary: laya", 1)
	if err := os.WriteFile(filepath.Join(directory, "test.yaml"), []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := provider.NewSystemOne(provider.SystemOneConfig{Name: "laya", BaseURL: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(definitions, engine.New(provider.NewSet(adapter)), 1024, 1, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodPost, "/v1/decisions/test/versions/v1:execute", strings.NewReader(`{"message":"hello"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{`"choice":"yes"`, `"provider":"laya"`, `"yes":0.95`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("body = %s; missing %s", response.Body.String(), expected)
		}
	}
}
