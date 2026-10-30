package definition

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = "v1alpha1"

type Definition struct {
	SchemaVersion string   `yaml:"schemaVersion" json:"schemaVersion"`
	Metadata      Metadata `yaml:"metadata" json:"metadata"`
	Spec          Spec     `yaml:"spec" json:"spec"`
}

type Metadata struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

type Spec struct {
	Description string                    `yaml:"description" json:"description"`
	Input       Schema                    `yaml:"input" json:"input"`
	Decisions   map[string]ChoiceDecision `yaml:"decisions" json:"decisions"`
	Routing     Routing                   `yaml:"routing" json:"routing"`
	Review      Review                    `yaml:"review" json:"review"`
}

type ChoiceDecision struct {
	Type        string            `yaml:"type" json:"type"`
	Instruction string            `yaml:"instruction" json:"instruction"`
	Options     map[string]string `yaml:"options" json:"options"`
}

type Routing struct {
	Primary  string `yaml:"primary" json:"primary"`
	Fallback string `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	Timeout  string `yaml:"timeout" json:"timeout"`
}

type Review struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type Schema struct {
	Type                 string             `yaml:"type" json:"type"`
	AdditionalProperties *bool              `yaml:"additionalProperties,omitempty" json:"additionalProperties,omitempty"`
	Required             []string           `yaml:"required,omitempty" json:"required,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty" json:"properties,omitempty"`
	Items                *Schema            `yaml:"items,omitempty" json:"items,omitempty"`
	Enum                 []any              `yaml:"enum,omitempty" json:"enum,omitempty"`
	MinLength            *int               `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength            *int               `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Minimum              *float64           `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum              *float64           `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	MinItems             *int               `yaml:"minItems,omitempty" json:"minItems,omitempty"`
	MaxItems             *int               `yaml:"maxItems,omitempty" json:"maxItems,omitempty"`
}

func (d *Definition) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schemaVersion must be %q", SchemaVersion)
	}
	if strings.TrimSpace(d.Metadata.Name) == "" || strings.Contains(d.Metadata.Name, "/") {
		return fmt.Errorf("metadata.name must be nonempty and cannot contain '/'")
	}
	if strings.TrimSpace(d.Metadata.Version) == "" || strings.Contains(d.Metadata.Version, "/") {
		return fmt.Errorf("metadata.version must be nonempty and cannot contain '/'")
	}
	if err := d.Spec.Input.ValidateDefinition("spec.input"); err != nil {
		return err
	}
	if len(d.Spec.Decisions) == 0 {
		return fmt.Errorf("spec.decisions must contain at least one decision")
	}
	for name, decision := range d.Spec.Decisions {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("spec.decisions contains an empty name")
		}
		if decision.Type != "choice" {
			return fmt.Errorf("spec.decisions.%s.type must be choice", name)
		}
		if strings.TrimSpace(decision.Instruction) == "" {
			return fmt.Errorf("spec.decisions.%s.instruction must be nonempty", name)
		}
		if len(decision.Options) < 2 || len(decision.Options) > 32 {
			return fmt.Errorf("spec.decisions.%s.options must contain 2-32 choices", name)
		}
		for id, description := range decision.Options {
			if strings.TrimSpace(id) == "" || strings.TrimSpace(description) == "" {
				return fmt.Errorf("spec.decisions.%s.options must have nonempty IDs and descriptions", name)
			}
		}
	}
	if strings.TrimSpace(d.Spec.Routing.Primary) == "" {
		return fmt.Errorf("spec.routing.primary must be nonempty")
	}
	timeout, err := time.ParseDuration(d.Spec.Routing.Timeout)
	if err != nil || timeout <= 0 {
		return fmt.Errorf("spec.routing.timeout must be a positive duration")
	}
	return nil
}

func (d *Definition) Timeout() time.Duration {
	timeout, _ := time.ParseDuration(d.Spec.Routing.Timeout)
	return timeout
}

func (d *Definition) DecisionNames() []string {
	names := make([]string, 0, len(d.Spec.Decisions))
	for name := range d.Spec.Decisions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
