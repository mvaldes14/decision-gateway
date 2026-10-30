package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mvaldes14/decision-gateway/internal/definition"
	"github.com/mvaldes14/decision-gateway/internal/provider"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

type FallbackError struct {
	Primary  error
	Fallback error
}

func (e *FallbackError) Error() string {
	return fmt.Sprintf("primary failed: %v; fallback failed: %v", e.Primary, e.Fallback)
}

func (e *FallbackError) Unwrap() []error { return []error{e.Primary, e.Fallback} }

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

func (e *Engine) ValidateRoutes(definitions []*definition.Definition) error {
	unavailable := make([]string, 0)
	for _, decision := range definitions {
		primaryAvailable := e.providers.Has(decision.Spec.Routing.Primary)
		fallbackAvailable := decision.Spec.Routing.Fallback != "" && e.providers.Has(decision.Spec.Routing.Fallback)
		if !primaryAvailable && !fallbackAvailable {
			unavailable = append(unavailable, fmt.Sprintf("%s/%s", decision.Metadata.Name, decision.Metadata.Version))
		}
	}
	if len(unavailable) > 0 {
		return fmt.Errorf("no configured provider available for decisions: %s", strings.Join(unavailable, ", "))
	}
	return nil
}

func (e *Engine) Execute(ctx context.Context, decision *definition.Definition, input map[string]any) (result Result, executeErr error) {
	ctx, span := otel.Tracer("github.com/mvaldes14/decision-gateway/internal/engine").Start(ctx, "decision.execute")
	span.SetAttributes(
		attribute.String("decision.name", decision.Metadata.Name),
		attribute.String("decision.version", decision.Metadata.Version),
		attribute.String("decision.provider.primary", decision.Spec.Routing.Primary),
	)
	defer func() {
		if executeErr != nil {
			span.RecordError(executeErr)
			span.SetStatus(codes.Error, executeErr.Error())
		} else {
			span.SetAttributes(
				attribute.String("decision.provider.used", result.Provider),
				attribute.Bool("decision.fallback.used", result.FallbackUsed),
			)
		}
		span.End()
	}()

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
		return Result{}, &ExecutionError{Kind: ErrorProvider, Err: &FallbackError{Primary: err, Fallback: fallbackErr}}
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
		return Result{}, provider.NewError(adapter.Name(), provider.ErrorInvalidResponse, 0, err)
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
