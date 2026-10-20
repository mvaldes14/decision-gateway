package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mvaldes14/decision-gateway/internal/definition"
	"github.com/mvaldes14/decision-gateway/internal/engine"
	"github.com/mvaldes14/decision-gateway/internal/registry"
)

type requestIDKey struct{}

type Server struct {
	registry     *registry.Registry
	engine       *engine.Engine
	maxBodyBytes int64
	concurrency  chan struct{}
	logger       *slog.Logger
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type executionResponse struct {
	Decision       string                    `json:"decision"`
	Version        string                    `json:"version"`
	Results        map[string]decisionResult `json:"results"`
	Provider       string                    `json:"provider"`
	ReviewRequired bool                      `json:"review_required"`
	RequestID      string                    `json:"request_id"`
}

type decisionResult struct {
	Choice string             `json:"choice"`
	Scores map[string]float64 `json:"scores,omitempty"`
}

func New(registry *registry.Registry, executionEngine *engine.Engine, maxBodyBytes int64, maxConcurrent int, logger *slog.Logger) http.Handler {
	server := &Server{
		registry: registry, engine: executionEngine, maxBodyBytes: maxBodyBytes,
		concurrency: make(chan struct{}, maxConcurrent), logger: logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /readyz", server.ready)
	mux.HandleFunc("GET /v1/decisions", server.list)
	mux.HandleFunc("/v1/decisions/", server.decision)
	return server.requestIDs(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"decisions": s.registry.List()})
}

func (s *Server) decision(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/decisions/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] != "versions" || parts[2] == "" {
		s.writeError(w, r, http.StatusNotFound, "DECISION_NOT_FOUND", "decision not found")
		return
	}

	name := parts[0]
	version := parts[2]
	execute := strings.HasSuffix(version, ":execute")
	version = strings.TrimSuffix(version, ":execute")
	value, err := s.registry.Get(name, version)
	if err != nil {
		s.writeError(w, r, http.StatusNotFound, "DECISION_NOT_FOUND", "decision not found")
		return
	}

	switch {
	case r.Method == http.MethodGet && !execute:
		s.describe(w, value)
	case r.Method == http.MethodPost && execute:
		s.execute(w, r, value)
	default:
		w.Header().Set("Allow", allowedMethods(execute))
		s.writeError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (s *Server) describe(w http.ResponseWriter, value *definition.Definition) {
	outputProperties := make(map[string]any, len(value.Spec.Decisions))
	for name, choice := range value.Spec.Decisions {
		options := make([]string, 0, len(choice.Options))
		for option := range choice.Options {
			options = append(options, option)
		}
		outputProperties[name] = map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"choice": map[string]any{"type": "string", "enum": options}},
			"required":             []string{"choice"},
			"additionalProperties": false,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":         value.Metadata.Name,
		"version":      value.Metadata.Version,
		"description":  value.Spec.Description,
		"input_schema": value.Spec.Input,
		"output_schema": map[string]any{
			"type": "object", "properties": outputProperties,
			"required": value.DecisionNames(), "additionalProperties": false,
		},
	})
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request, value *definition.Definition) {
	select {
	case s.concurrency <- struct{}{}:
		defer func() { <-s.concurrency }()
	default:
		s.writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "execution capacity exhausted")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var input map[string]any
	if err := decoder.Decode(&input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "request body must contain one JSON object")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "request body must contain one JSON object")
		return
	}
	if input == nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_INPUT", "input must be object")
		return
	}
	if err := value.Spec.Input.Validate(input); err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_INPUT", err.Error())
		return
	}

	result, err := s.engine.Execute(r.Context(), value, input)
	if err != nil {
		if engine.Kind(err) == engine.ErrorTimeout {
			s.writeError(w, r, http.StatusGatewayTimeout, "PROVIDER_TIMEOUT", "execution exceeded timeout")
			return
		}
		s.logger.Warn("decision execution failed", "request_id", requestID(r.Context()), "decision", value.Metadata.Name, "version", value.Metadata.Version, "error", err)
		s.writeError(w, r, http.StatusBadGateway, "PROVIDER_ERROR", "no provider returned a valid result")
		return
	}

	results := make(map[string]decisionResult, len(result.Choices))
	for name, choice := range result.Choices {
		results[name] = decisionResult{Choice: choice, Scores: result.Scores[name]}
	}
	writeJSON(w, http.StatusOK, executionResponse{
		Decision: value.Metadata.Name, Version: value.Metadata.Version, Results: results,
		Provider: result.Provider, ReviewRequired: false, RequestID: requestID(r.Context()),
	})
}

func (s *Server) requestIDs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message, RequestID: requestID(r.Context())}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("unexpected trailing JSON")
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

func allowedMethods(execute bool) string {
	if execute {
		return http.MethodPost
	}
	return http.MethodGet
}
