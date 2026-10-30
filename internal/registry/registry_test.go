package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validDefinition = `schemaVersion: v1alpha1
metadata: {name: test, version: v1}
spec:
  description: test decision
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
  routing: {primary: fake, timeout: 1s}
  review: {enabled: false}
`

func TestLoad(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "test.yaml"), []byte(validDefinition), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	value, err := registry.Get("test", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if value.Spec.Description != "test decision" {
		t.Fatalf("description = %q", value.Spec.Description)
	}
}

func TestLoadRejectsLegacyKubernetesStyleFields(t *testing.T) {
	directory := t.TempDir()
	legacy := strings.Replace(validDefinition, "schemaVersion: v1alpha1", "apiVersion: decisiongateway.dev/v1alpha1\nkind: Decision", 1)
	if err := os.WriteFile(filepath.Join(directory, "legacy.yaml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(directory)
	if err == nil || !strings.Contains(err.Error(), "field apiVersion not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsUnsupportedSchemaVersion(t *testing.T) {
	directory := t.TempDir()
	invalid := strings.Replace(validDefinition, "schemaVersion: v1alpha1", "schemaVersion: v2", 1)
	if err := os.WriteFile(filepath.Join(directory, "invalid.yaml"), []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(directory)
	if err == nil || !strings.Contains(err.Error(), `schemaVersion must be "v1alpha1"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	directory := t.TempDir()
	invalid := strings.Replace(validDefinition, "  description:", "  unsupported: true\n  description:", 1)
	if err := os.WriteFile(filepath.Join(directory, "invalid.yaml"), []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(directory)
	if err == nil || !strings.Contains(err.Error(), "field unsupported not found") {
		t.Fatalf("error = %v", err)
	}
}
