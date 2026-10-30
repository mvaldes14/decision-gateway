package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemOneExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		var payload systemOneRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "clef" {
			t.Fatalf("model = %q", payload.Model)
		}
		if payload.State["title"] != "broken checkout" {
			t.Fatalf("state = %#v", payload.State)
		}
		question := payload.Questions["project"]
		if question.Type != "choice" || question.Criteria["platform"] == "" {
			t.Fatalf("question = %#v", question)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"clef","answers":{"project":{"type":"choice","choice":"platform","probabilities":{"platform":0.9,"application":0.1},"confidence":0.53}},"usage":{"input_tokens":10,"output_tokens":1}}`))
	}))
	defer server.Close()

	adapter, err := NewSystemOne(SystemOneConfig{Name: "clef", BaseURL: server.URL + "/", APIKey: "secret", Model: "clef"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), Request{
		Input: map[string]any{"title": "broken checkout"},
		Decisions: []ChoiceQuestion{{
			Name: "project", Instruction: "Choose owner", Options: map[string]string{
				"platform": "Infrastructure", "application": "Application code",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Choices["project"] != "platform" || result.Scores["project"]["platform"] != 0.9 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSystemOneOmitsOptionalModelAndAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("authorization = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["model"]; ok {
			t.Fatalf("payload contains model: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"answers":{"label":{"type":"choice","choice":"yes","probabilities":{"yes":1}}}}`))
	}))
	defer server.Close()

	adapter, err := NewSystemOne(SystemOneConfig{Name: "laya", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Execute(context.Background(), Request{Decisions: []ChoiceQuestion{{Name: "label", Options: map[string]string{"yes": "Yes", "no": "No"}}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSystemOneHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
	}))
	defer server.Close()

	adapter, err := NewSystemOne(SystemOneConfig{Name: "clef", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), Request{}); err == nil || err.Error() != "clef returned HTTP 401: invalid API key" {
		t.Fatalf("error = %v", err)
	}
}

func TestSystemOneRejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewSystemOne(SystemOneConfig{Name: "laya", BaseURL: "localhost:8000"}); err == nil {
		t.Fatal("expected invalid URL error")
	}
}
