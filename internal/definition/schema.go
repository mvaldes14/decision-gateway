package definition

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

var supportedTypes = map[string]bool{
	"object": true, "string": true, "number": true, "integer": true,
	"boolean": true, "array": true,
}

func (s *Schema) ValidateDefinition(path string) error {
	if !supportedTypes[s.Type] {
		return fmt.Errorf("%s.type %q is unsupported", path, s.Type)
	}
	if s.MinLength != nil && *s.MinLength < 0 || s.MaxLength != nil && *s.MaxLength < 0 {
		return fmt.Errorf("%s length constraints must be nonnegative", path)
	}
	if s.MinLength != nil && s.MaxLength != nil && *s.MinLength > *s.MaxLength {
		return fmt.Errorf("%s.minLength cannot exceed maxLength", path)
	}
	if s.Minimum != nil && s.Maximum != nil && *s.Minimum > *s.Maximum {
		return fmt.Errorf("%s.minimum cannot exceed maximum", path)
	}
	if s.MinItems != nil && *s.MinItems < 0 || s.MaxItems != nil && *s.MaxItems < 0 {
		return fmt.Errorf("%s item constraints must be nonnegative", path)
	}
	if s.MinItems != nil && s.MaxItems != nil && *s.MinItems > *s.MaxItems {
		return fmt.Errorf("%s.minItems cannot exceed maxItems", path)
	}

	switch s.Type {
	case "object":
		required := make(map[string]bool, len(s.Required))
		for _, name := range s.Required {
			if required[name] {
				return fmt.Errorf("%s.required contains duplicate %q", path, name)
			}
			required[name] = true
			if _, ok := s.Properties[name]; !ok {
				return fmt.Errorf("%s.required references unknown property %q", path, name)
			}
		}
		for name, property := range s.Properties {
			if property == nil {
				return fmt.Errorf("%s.properties.%s cannot be null", path, name)
			}
			if err := property.ValidateDefinition(path + ".properties." + name); err != nil {
				return err
			}
		}
	case "array":
		if s.Items == nil {
			return fmt.Errorf("%s.items is required for arrays", path)
		}
		if err := s.Items.ValidateDefinition(path + ".items"); err != nil {
			return err
		}
	}

	return nil
}

func (s *Schema) Validate(value any) error {
	return s.validate(value, "input")
}

func (s *Schema) validate(value any, path string) error {
	if !matchesType(s.Type, value) {
		return fmt.Errorf("%s must be %s", path, s.Type)
	}
	if len(s.Enum) > 0 {
		matched := false
		for _, allowed := range s.Enum {
			if reflect.DeepEqual(normalizeNumber(value), normalizeNumber(allowed)) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s must be one of configured enum values", path)
		}
	}

	switch s.Type {
	case "object":
		object := value.(map[string]any)
		for _, name := range s.Required {
			if _, ok := object[name]; !ok {
				return fmt.Errorf("%s.%s is required", path, name)
			}
		}
		for name, child := range object {
			property, ok := s.Properties[name]
			if !ok {
				if s.AdditionalProperties != nil && !*s.AdditionalProperties {
					return fmt.Errorf("%s.%s is not allowed", path, name)
				}
				continue
			}
			if err := property.validate(child, path+"."+name); err != nil {
				return err
			}
		}
	case "array":
		items := value.([]any)
		if s.MinItems != nil && len(items) < *s.MinItems {
			return fmt.Errorf("%s must contain at least %d items", path, *s.MinItems)
		}
		if s.MaxItems != nil && len(items) > *s.MaxItems {
			return fmt.Errorf("%s must contain at most %d items", path, *s.MaxItems)
		}
		for i, item := range items {
			if err := s.Items.validate(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case "string":
		text := value.(string)
		length := len([]rune(text))
		if s.MinLength != nil && length < *s.MinLength {
			return fmt.Errorf("%s must contain at least %d characters", path, *s.MinLength)
		}
		if s.MaxLength != nil && length > *s.MaxLength {
			return fmt.Errorf("%s must contain at most %d characters", path, *s.MaxLength)
		}
	case "number", "integer":
		number := toFloat(value)
		if s.Minimum != nil && number < *s.Minimum {
			return fmt.Errorf("%s must be at least %v", path, *s.Minimum)
		}
		if s.Maximum != nil && number > *s.Maximum {
			return fmt.Errorf("%s must be at most %v", path, *s.Maximum)
		}
	}
	return nil
}

func matchesType(kind string, value any) bool {
	switch kind {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case json.Number, float64, float32, int, int64:
			return true
		}
	case "integer":
		n := toFloat(value)
		return !math.IsNaN(n) && math.Trunc(n) == n
	}
	return false
}

func toFloat(value any) float64 {
	switch number := value.(type) {
	case json.Number:
		result, err := number.Float64()
		if err == nil {
			return result
		}
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	}
	return math.NaN()
}

func normalizeNumber(value any) any {
	if number, ok := value.(json.Number); ok {
		if strings.Contains(number.String(), ".") {
			result, _ := number.Float64()
			return result
		}
		result, _ := number.Int64()
		return result
	}
	return value
}
