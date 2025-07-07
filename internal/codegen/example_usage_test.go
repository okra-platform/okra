package codegen_test

import (
	"fmt"
	"log"

	"github.com/okra-platform/okra/internal/codegen"
	"github.com/okra-platform/okra/internal/codegen/golang"
	"github.com/okra-platform/okra/internal/codegen/typescript"
	"github.com/okra-platform/okra/internal/schema"
)

func Example_usage() {
	// Create a sample schema
	s := &schema.Schema{
		Enums: []schema.EnumType{
			{
				Name: "Status",
				Values: []schema.EnumValue{
					{Name: "Active"},
					{Name: "Inactive"},
				},
			},
		},
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
					{Name: "name", Type: "String", Required: true},
					{Name: "status", Type: "Status", Required: true},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "UserService",
				Methods: []schema.Method{
					{
						Name:       "getUser",
						InputType:  "String",
						OutputType: "User",
					},
				},
			},
		},
	}

	// Method 1: Direct usage
	goGen := golang.NewGenerator("userapi")
	goCode, err := goGen.Generate(s)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Go code generated successfully")

	tsGen := typescript.NewGenerator("UserAPI")
	tsCode, err := tsGen.Generate(s)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("TypeScript code generated successfully")

	// Method 2: Using registry
	registry := codegen.NewRegistry()
	registry.Register("go", func(pkg string) codegen.Generator {
		return golang.NewGenerator(pkg)
	})
	registry.Register("typescript", func(pkg string) codegen.Generator {
		return typescript.NewGenerator(pkg)
	})

	// Generate for multiple languages
	languages := []string{"go", "typescript"}
	for _, lang := range languages {
		gen, err := registry.Get(lang, "myapi")
		if err != nil {
			log.Fatal(err)
		}

		code, err := gen.Generate(s)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Generated %s code (%s)\n", gen.Language(), gen.FileExtension())
		_ = code // Use the generated code
	}

	// Output:
	// Go code generated successfully
	// TypeScript code generated successfully
	// Generated go code (.go)
	// Generated typescript code (.ts)

	_ = goCode
	_ = tsCode
}