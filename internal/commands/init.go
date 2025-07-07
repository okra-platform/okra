package commands

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
)

//go:embed templates/*
var templatesFS embed.FS

type InitOptions struct {
	ProjectName string
	Template    string
}

func (c *Controller) Init(ctx context.Context) error {
	if err := checkScaffoldInstalled(); err != nil {
		return fmt.Errorf("scaffold is not installed: %w\n\nPlease install scaffold from https://github.com/hay-kot/scaffold", err)
	}

	options, err := promptInitOptions()
	if err != nil {
		return fmt.Errorf("failed to get init options: %w", err)
	}

	templatesDir, err := ensureTemplatesExtracted()
	if err != nil {
		return fmt.Errorf("failed to extract templates: %w", err)
	}

	templatePath := filepath.Join(templatesDir, options.Template)
	if err := runScaffold(options.ProjectName, templatePath); err != nil {
		return fmt.Errorf("failed to scaffold project: %w", err)
	}

	fmt.Printf("✅ Successfully created %s project: %s\n", options.Template, options.ProjectName)
	return nil
}

func checkScaffoldInstalled() error {
	_, err := exec.LookPath("scaffold")
	return err
}

func promptInitOptions() (*InitOptions, error) {
	var projectName string
	var template string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Name of your new OKRA project").
				Value(&projectName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("project name cannot be empty")
					}
					if _, err := os.Stat(s); err == nil {
						return fmt.Errorf("directory %s already exists", s)
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("Template").
				Description("Choose a project template").
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("TypeScript", "typescript"),
				).
				Value(&template),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return &InitOptions{
		ProjectName: projectName,
		Template:    template,
	}, nil
}

func ensureTemplatesExtracted() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	okraDir := filepath.Join(homeDir, ".okra", "templates")
	
	if _, err := os.Stat(okraDir); err == nil {
		return okraDir, nil
	}

	if err := os.MkdirAll(okraDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create templates directory: %w", err)
	}

	if err := extractTemplates(okraDir); err != nil {
		return "", fmt.Errorf("failed to extract templates: %w", err)
	}

	return okraDir, nil
}

func extractTemplates(destDir string) error {
	return fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "templates" {
			return nil
		}

		relPath, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

func runScaffold(projectName, templatePath string) error {
	cmd := exec.Command("scaffold", "new", projectName, "--template", templatePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}