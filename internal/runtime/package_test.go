package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/okra-platform/okra/internal/config"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/okra-platform/okra/internal/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Test plan for ServicePackage:
// 1. Test NewServicePackage with valid inputs
// 2. Test NewServicePackage with nil module (error case)
// 3. Test NewServicePackage with nil schema (error case)
// 4. Test NewServicePackage with nil config (error case)
// 5. Test NewServicePackage with empty services (error case)
// 6. Test GetMethod with existing method
// 7. Test GetMethod with non-existing method
// 8. Test ServicePackage with FileDescriptors

// Mock implementations for testing
type mockWASMCompiledModule struct{}

func (m *mockWASMCompiledModule) Instantiate(ctx context.Context) (wasm.WASMWorker, error) {
	return nil, nil
}

func (m *mockWASMCompiledModule) Close(ctx context.Context) error {
	return nil
}

func TestServicePackage_NewServicePackage(t *testing.T) {
	// Test: Create a valid service package
	validModule := &mockWASMCompiledModule{}
	validSchema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{Name: "Method1"},
					{Name: "Method2"},
				},
			},
		},
	}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(validModule, validSchema, validConfig)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	assert.Equal(t, validModule, pkg.Module)
	assert.Equal(t, validSchema, pkg.Schema)
	assert.Equal(t, validConfig, pkg.Config)
	assert.Equal(t, "TestService", pkg.ServiceName)
	assert.Len(t, pkg.Methods, 2)
	assert.Nil(t, pkg.FileDescriptors) // Should be nil by default
}

func TestServicePackage_NewServicePackageWithNilModule(t *testing.T) {
	// Test: Nil module should return error
	validSchema := &schema.Schema{
		Services: []schema.Service{{Name: "Test"}},
	}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(nil, validSchema, validConfig)
	assert.Error(t, err)
	assert.Equal(t, ErrNilModule, err)
	assert.Nil(t, pkg)
}

func TestServicePackage_NewServicePackageWithNilSchema(t *testing.T) {
	// Test: Nil schema should return error
	validModule := &mockWASMCompiledModule{}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(validModule, nil, validConfig)
	assert.Error(t, err)
	assert.Equal(t, ErrNilSchema, err)
	assert.Nil(t, pkg)
}

func TestServicePackage_NewServicePackageWithNilConfig(t *testing.T) {
	// Test: Nil config should return error
	validModule := &mockWASMCompiledModule{}
	validSchema := &schema.Schema{
		Services: []schema.Service{{Name: "Test"}},
	}

	pkg, err := NewServicePackage(validModule, validSchema, nil)
	assert.Error(t, err)
	assert.Equal(t, ErrNilConfig, err)
	assert.Nil(t, pkg)
}

func TestServicePackage_NewServicePackageWithEmptyServices(t *testing.T) {
	// Test: Empty services should return error
	validModule := &mockWASMCompiledModule{}
	emptySchema := &schema.Schema{
		Services: []schema.Service{},
	}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(validModule, emptySchema, validConfig)
	assert.Error(t, err)
	assert.Equal(t, ErrNoServices, err)
	assert.Nil(t, pkg)
}

func TestServicePackage_GetMethod(t *testing.T) {
	// Test: GetMethod returns existing method
	validModule := &mockWASMCompiledModule{}
	validSchema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{Name: "Method1", InputType: "Input1", OutputType: "Output1"},
					{Name: "Method2", InputType: "Input2", OutputType: "Output2"},
				},
			},
		},
	}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(validModule, validSchema, validConfig)
	require.NoError(t, err)

	// Test existing method
	method, ok := pkg.GetMethod("Method1")
	assert.True(t, ok)
	assert.NotNil(t, method)
	assert.Equal(t, "Method1", method.Name)
	assert.Equal(t, "Input1", method.InputType)
	assert.Equal(t, "Output1", method.OutputType)

	// Test non-existing method
	method, ok = pkg.GetMethod("NonExistentMethod")
	assert.False(t, ok)
	assert.Nil(t, method)
}


func strPtr(s string) *string {
	return &s
}

func TestServicePackage_WithFileDescriptors(t *testing.T) {
	// Test: WithFileDescriptors method
	validModule := &mockWASMCompiledModule{}
	validSchema := &schema.Schema{
		Services: []schema.Service{
			{Name: "TestService", Methods: []schema.Method{{Name: "Method1"}}},
		},
	}
	validConfig := &config.Config{}

	pkg, err := NewServicePackage(validModule, validSchema, validConfig)
	require.NoError(t, err)
	require.Nil(t, pkg.FileDescriptors)

	// Add FileDescriptors using WithFileDescriptors
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{Name: strPtr("test2.proto")},
		},
	}
	
	result := pkg.WithFileDescriptors(fds)
	assert.Equal(t, pkg, result) // Should return the same instance
	assert.NotNil(t, pkg.FileDescriptors)
	assert.Len(t, pkg.FileDescriptors.File, 1)
	assert.Equal(t, "test2.proto", *pkg.FileDescriptors.File[0].Name)
}

func TestLoadFileDescriptors(t *testing.T) {
	// Test: LoadFileDescriptors from file
	// Create a temporary file with a valid FileDescriptorSet
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
			},
		},
	}
	
	data, err := proto.Marshal(fds)
	require.NoError(t, err)
	
	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "test-*.pb.desc")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	_, err = tmpFile.Write(data)
	require.NoError(t, err)
	err = tmpFile.Close()
	require.NoError(t, err)
	
	// Load from file
	loaded, err := LoadFileDescriptors(tmpFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Len(t, loaded.File, 1)
	assert.Equal(t, "test.proto", *loaded.File[0].Name)
	assert.Equal(t, "testpkg", *loaded.File[0].Package)
}

func TestLoadFileDescriptors_FileNotFound(t *testing.T) {
	// Test: LoadFileDescriptors with non-existent file
	loaded, err := LoadFileDescriptors("/non/existent/file.pb.desc")
	assert.Error(t, err)
	assert.Nil(t, loaded)
}

func TestLoadFileDescriptors_InvalidData(t *testing.T) {
	// Test: LoadFileDescriptors with invalid protobuf data
	tmpFile, err := os.CreateTemp("", "invalid-*.pb.desc")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	
	_, err = tmpFile.Write([]byte("invalid protobuf data"))
	require.NoError(t, err)
	err = tmpFile.Close()
	require.NoError(t, err)
	
	loaded, err := LoadFileDescriptors(tmpFile.Name())
	assert.Error(t, err)
	assert.Nil(t, loaded)
}