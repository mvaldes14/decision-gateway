package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvaldes14/decision-gateway/internal/definition"
	"gopkg.in/yaml.v3"
)

var ErrNotFound = errors.New("decision not found")

type Registry struct {
	definitions map[string]map[string]*definition.Definition
}

type Summary struct {
	Name        string   `json:"name"`
	Versions    []string `json:"versions"`
	Description string   `json:"description"`
}

func Load(directory string) (*Registry, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read definitions directory %q: %w", directory, err)
	}

	registry := &Registry{definitions: make(map[string]map[string]*definition.Definition)}
	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".yaml" && extension != ".yml" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if err := registry.loadFile(path); err != nil {
			return nil, err
		}
		loaded++
	}
	if loaded == 0 {
		return nil, fmt.Errorf("no YAML decision definitions found in %q", directory)
	}
	return registry, nil
}

func (r *Registry) loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open definition %q: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var value definition.Definition
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode definition %q: %w", path, err)
	}
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate definition %q: %w", path, err)
	}

	versions, ok := r.definitions[value.Metadata.Name]
	if !ok {
		versions = make(map[string]*definition.Definition)
		r.definitions[value.Metadata.Name] = versions
	}
	if _, exists := versions[value.Metadata.Version]; exists {
		return fmt.Errorf("duplicate decision %s/%s", value.Metadata.Name, value.Metadata.Version)
	}
	versions[value.Metadata.Version] = &value
	return nil
}

func (r *Registry) Get(name, version string) (*definition.Definition, error) {
	versions, ok := r.definitions[name]
	if !ok {
		return nil, ErrNotFound
	}
	value, ok := versions[version]
	if !ok {
		return nil, ErrNotFound
	}
	return value, nil
}

func (r *Registry) All() []*definition.Definition {
	values := make([]*definition.Definition, 0)
	for _, versions := range r.definitions {
		for _, value := range versions {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Metadata.Name == values[j].Metadata.Name {
			return values[i].Metadata.Version < values[j].Metadata.Version
		}
		return values[i].Metadata.Name < values[j].Metadata.Name
	})
	return values
}

func (r *Registry) List() []Summary {
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Summary, 0, len(names))
	for _, name := range names {
		versions := make([]string, 0, len(r.definitions[name]))
		for version := range r.definitions[name] {
			versions = append(versions, version)
		}
		sort.Strings(versions)
		description := r.definitions[name][versions[len(versions)-1]].Spec.Description
		result = append(result, Summary{Name: name, Versions: versions, Description: description})
	}
	return result
}

func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
