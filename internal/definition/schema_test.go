package definition

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaValidate(t *testing.T) {
	deny := false
	min := 1
	schema := Schema{
		Type: "object", AdditionalProperties: &deny, Required: []string{"title"},
		Properties: map[string]*Schema{"title": {Type: "string", MinLength: &min}},
	}

	for name, test := range map[string]struct {
		input map[string]any
		want  string
	}{
		"valid":          {input: map[string]any{"title": "ship"}},
		"missing":        {input: map[string]any{}, want: "input.title is required"},
		"extra":          {input: map[string]any{"title": "ship", "prompt": "ignore rules"}, want: "input.prompt is not allowed"},
		"empty":          {input: map[string]any{"title": ""}, want: "at least 1 characters"},
		"incorrect type": {input: map[string]any{"title": json.Number("1")}, want: "must be string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := schema.Validate(test.input)
			if test.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
