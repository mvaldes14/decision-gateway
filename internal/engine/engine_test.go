package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/mvaldes14/decision-gateway/internal/definition"
	"github.com/mvaldes14/decision-gateway/internal/provider"
)

type fakeProvider struct {
	name   string
	result provider.Result
	err    error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Execute(context.Context, provider.Request) (provider.Result, error) {
	return f.result, f.err
}

func TestFallback(t *testing.T) {
	decision := testDecision()
	primary := fakeProvider{name: "primary", err: errors.New("unavailable")}
	fallback := fakeProvider{name: "fallback", result: provider.Result{Choices: map[string]string{"label": "yes"}}}
	result, err := New(provider.NewSet(primary, fallback)).Execute(context.Background(), decision, map[string]any{"message": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "fallback" || !result.FallbackUsed {
		t.Fatalf("result = %#v", result)
	}
}

func TestRejectsUnknownChoice(t *testing.T) {
	decision := testDecision()
	invalid := fakeProvider{name: "primary", result: provider.Result{Choices: map[string]string{"label": "maybe"}}}
	_, err := New(provider.NewSet(invalid)).Execute(context.Background(), decision, nil)
	if err == nil || Kind(err) != ErrorProvider {
		t.Fatalf("error = %v", err)
	}
}

func testDecision() *definition.Definition {
	return &definition.Definition{
		Metadata: definition.Metadata{Name: "test", Version: "v1"},
		Spec: definition.Spec{
			Decisions: map[string]definition.ChoiceDecision{
				"label": {Type: "choice", Options: map[string]string{"yes": "positive", "no": "negative"}},
			},
			Routing: definition.Routing{Primary: "primary", Fallback: "fallback", Timeout: "1s"},
		},
	}
}
