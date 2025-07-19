package codegen

import (
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGenerator is a test generator
type mockGenerator struct {
	lang string
}

func (m *mockGenerator) Generate(s *schema.Schema) ([]byte, error) {
	return []byte("mock output"), nil
}

func (m *mockGenerator) Language() string {
	return m.lang
}

func (m *mockGenerator) FileExtension() string {
	return ".mock"
}

func TestRegistry_NewRegistry(t *testing.T) {
	// Test: New registry is empty by default
	r := NewRegistry()
	assert.NotNil(t, r)

	// Should error on unknown language
	_, err := r.Get("unknown", "test")
	assert.Error(t, err)
}

func TestRegistry_Register(t *testing.T) {
	// Test: Register custom generator
	r := NewRegistry()

	// Register a mock generator
	r.Register("mock", func(packageName string) Generator {
		return &mockGenerator{lang: "mock"}
	})

	// Get the registered generator
	gen, err := r.Get("mock", "testpkg")
	require.NoError(t, err)
	assert.NotNil(t, gen)
	assert.Equal(t, "mock", gen.Language())
}

func TestRegistry_UnsupportedLanguage(t *testing.T) {
	// Test: Error for unsupported language
	r := NewRegistry()

	gen, err := r.Get("unknown", "testpkg")
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "unsupported language: unknown")
}

func TestRegistry_Languages(t *testing.T) {
	// Test: List of supported languages
	r := NewRegistry()

	// Empty registry should have no languages
	languages := r.Languages()
	assert.Empty(t, languages)

	// Register some languages
	r.Register("go", func(packageName string) Generator {
		return &mockGenerator{lang: "go"}
	})
	r.Register("typescript", func(packageName string) Generator {
		return &mockGenerator{lang: "typescript"}
	})
	r.Register("python", func(packageName string) Generator {
		return &mockGenerator{lang: "python"}
	})

	languages = r.Languages()
	assert.Len(t, languages, 3)
	assert.Contains(t, languages, "go")
	assert.Contains(t, languages, "typescript")
	assert.Contains(t, languages, "python")
}
