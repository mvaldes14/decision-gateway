package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	maxErrorBodyBytes    = 4096
	maxResponseBodyBytes = 1 << 20
)

type SystemOneConfig struct {
	Name    string
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

type systemOneProvider struct {
	name     string
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

type systemOneRequest struct {
	Model     string                       `json:"model,omitempty"`
	State     map[string]any               `json:"state"`
	Questions map[string]systemOneQuestion `json:"questions"`
}

type systemOneQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type systemOneResponse struct {
	Answers map[string]systemOneAnswer `json:"answers"`
}

type systemOneAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type systemOneError struct {
	Error string `json:"error"`
}

func NewSystemOne(config SystemOneConfig) (Provider, error) {
	if strings.TrimSpace(config.Name) == "" {
		return nil, errors.New("system one provider name is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%s base URL must be an absolute URL", config.Name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s base URL scheme must be http or https", config.Name)
	}

	client := config.Client
	if client == nil {
		client = http.DefaultClient
	}
	return &systemOneProvider{
		name:     config.Name,
		endpoint: strings.TrimRight(parsed.String(), "/") + "/v1/systemone",
		apiKey:   config.APIKey,
		model:    config.Model,
		client:   client,
	}, nil
}

func (p *systemOneProvider) Name() string { return p.name }

func (p *systemOneProvider) Execute(ctx context.Context, request Request) (Result, error) {
	questions := make(map[string]systemOneQuestion, len(request.Decisions))
	for _, decision := range request.Decisions {
		questions[decision.Name] = systemOneQuestion{
			Type:         "choice",
			Instructions: decision.Instruction,
			Criteria:     decision.Options,
		}
	}
	payload, err := json.Marshal(systemOneRequest{Model: p.model, State: request.Input, Questions: questions})
	if err != nil {
		return Result{}, fmt.Errorf("encode %s request: %w", p.name, err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("create %s request: %w", p.name, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	if p.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return Result{}, fmt.Errorf("call %s: %w", p.name, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Result{}, providerHTTPError(p.name, response)
	}

	var decoded systemOneResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBodyBytes))
	if err := decoder.Decode(&decoded); err != nil {
		return Result{}, fmt.Errorf("decode %s response: %w", p.name, err)
	}

	result := Result{
		Choices: make(map[string]string, len(decoded.Answers)),
		Scores:  make(map[string]map[string]float64, len(decoded.Answers)),
	}
	for name, answer := range decoded.Answers {
		if answer.Type != "choice" {
			return Result{}, fmt.Errorf("%s returned unsupported answer type %q for %q", p.name, answer.Type, name)
		}
		if answer.Choice == "" {
			return Result{}, fmt.Errorf("%s returned empty choice for %q", p.name, name)
		}
		result.Choices[name] = answer.Choice
		if len(answer.Probabilities) > 0 {
			result.Scores[name] = answer.Probabilities
		}
	}
	return result, nil
}

func providerHTTPError(name string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	var envelope systemOneError
	if json.Unmarshal(body, &envelope) == nil && envelope.Error != "" {
		return fmt.Errorf("%s returned HTTP %d: %s", name, response.StatusCode, envelope.Error)
	}
	return fmt.Errorf("%s returned HTTP %d", name, response.StatusCode)
}
