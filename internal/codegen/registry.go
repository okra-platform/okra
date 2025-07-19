package codegen

import (
	"fmt"
)

// Registry manages available code generators
type Registry struct {
	generators map[string]func(packageName string) Generator
}

// NewRegistry creates a new generator registry
func NewRegistry() *Registry {
	r := &Registry{
		generators: make(map[string]func(packageName string) Generator),
	}
	return r
}

// Register adds a new generator factory to the registry
func (r *Registry) Register(language string, factory func(packageName string) Generator) {
	r.generators[language] = factory
}

// Get returns a generator for the specified language
func (r *Registry) Get(language, packageName string) (Generator, error) {
	factory, exists := r.generators[language]
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return factory(packageName), nil
}

// Languages returns a list of supported languages
func (r *Registry) Languages() []string {
	languages := make([]string, 0, len(r.generators))
	for lang := range r.generators {
		languages = append(languages, lang)
	}
	return languages
}
