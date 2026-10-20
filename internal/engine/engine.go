package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mvaldes14/decision-gateway/internal/definition"
	"github.com/mvaldes14/decision-gateway/internal/provider"
)

type ErrorKind string

const (
	ErrorProvider ErrorKind = "provider"
	ErrorTimeout  ErrorKind = "timeout"
)

type ExecutionError struct {
	Kind ErrorKind
	Err  error
}

func (e *ExecutionError) Error() string { return e.Err.Error() }
func (e *ExecutionError) Unwrap() error { return e.Err }

type Result struct {
	Choices      map[string]string
	Scores       map[string]map[string]float64
	Provider     string
	FallbackUsed bool
}

type Engine struct {
	providers *provider.Set
}

func New(providers *provider.Set) *Engine {
	return &Engine{providers: providers}
}

func (e *Engine) Execute(ctx context.Context, decision *definition.Definition, input map[string]any) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, decision.Timeout())
	defer cancel()

	request := makeRequest(decision, input)
	primaryResult, err := e.execute(ctx, decision, decision.Spec.Routing.Primary, request)
	if err == nil {
		return primaryResult, nil
	}
	if ctx.Err() != nil {
		return Result{}, &ExecutionError{Kind: ErrorTimeout, Err: ctx.Err()}
	}
	if decision.Spec.Routing.Fallback == "" {
		return Result{}, &ExecutionError{Kind: ErrorProvider, Err: err}
	}

	fallbackResult, fallbackErr := e.execute(ctx, decision, decision.Spec.Routing.Fallback, request)
	if fallbackErr != nil {
		if ctx.Err() != nil {
			return Result{}, &ExecutionError{Kind: ErrorTimeout, Err: ctx.Err()}
		}
		return Result{}, &ExecutionError{Kind: ErrorProvider, Err: fmt.Errorf("primary failed: %v; fallback failed: %w", err, fallbackErr)}
	}
	fallbackResult.FallbackUsed = true
	return fallbackResult, nil
}

func (e *Engine) execute(ctx context.Context, decision *definition.Definition, name string, request provider.Request) (Result, error) {
	adapter, err := e.providers.Get(name)
	if err != nil {
		return Result{}, err
	}
	value, err := adapter.Execute(ctx, request)
	if err != nil {
		return Result{}, err
	}
	if err := validateResult(decision, value); err != nil {
		return Result{}, err
	}
	return Result{Choices: value.Choices, Scores: value.Scores, Provider: adapter.Name()}, nil
}

func makeRequest(decision *definition.Definition, input map[string]any) provider.Request {
	names := decision.DecisionNames()
	questions := make([]provider.ChoiceQuestion, 0, len(names))
	for _, name := range names {
		choice := decision.Spec.Decisions[name]
		questions = append(questions, provider.ChoiceQuestion{
			Name: name, Instruction: choice.Instruction, Options: choice.Options,
		})
	}
	return provider.Request{Input: input, Decisions: questions}
}

func validateResult(decision *definition.Definition, value provider.Result) error {
	if len(value.Choices) != len(decision.Spec.Decisions) {
		return fmt.Errorf("provider returned %d choices; expected %d", len(value.Choices), len(decision.Spec.Decisions))
	}
	for name, choice := range value.Choices {
		configured, ok := decision.Spec.Decisions[name]
		if !ok {
			return fmt.Errorf("provider returned unknown decision %q", name)
		}
		if _, ok := configured.Options[choice]; !ok {
			return fmt.Errorf("provider returned invalid choice %q for %q", choice, name)
		}
	}
	return nil
}

func Kind(err error) ErrorKind {
	var executionError *ExecutionError
	if errors.As(err, &executionError) {
		return executionError.Kind
	}
	return ErrorProvider
}

func SortedScoreKeys(scores map[string]float64) []string {
	keys := make([]string, 0, len(scores))
	for key := range scores {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
