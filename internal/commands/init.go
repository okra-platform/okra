package commands

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

//go:embed templates/*
var templatesFS embed.FS

type InitOptions struct {
	ProjectName string
	Template    string
}

type ScaffoldRunner interface {
	CheckInstalled() error
	Run(projectName, templatePath string) error
}

type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	UserHomeDir() (string, error)
}

type scaffoldRunner struct{}

func (s *scaffoldRunner) CheckInstalled() error {
	_, err := exec.LookPath("scaffold")
	return err
}

func (s *scaffoldRunner) Run(projectName, templatePath string) error {
	cmd := exec.Command("scaffold", "new", projectName, "--template", templatePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type osFileSystem struct{}

func (fs *osFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *osFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (fs *osFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

type InitCommand struct {
	scaffold    ScaffoldRunner
	filesystem  FileSystem
	templatesFS fs.FS
	// For testing: if set, skip prompting
	testOptions *InitOptions
}

func NewInitCommand() *InitCommand {
	return &InitCommand{
		scaffold:    &scaffoldRunner{},
		filesystem:  &osFileSystem{},
		templatesFS: templatesFS,
	}
}

func (c *Controller) Init(ctx context.Context) error {
	cmd := NewInitCommand()
	return cmd.Run(ctx)
}

func (ic *InitCommand) Run(ctx context.Context) error {
	return ic.RunWithOptions(ctx)
}

func (ic *InitCommand) RunWithOptions(ctx context.Context, opts ...tea.ProgramOption) error {
	if err := ic.scaffold.CheckInstalled(); err != nil {
		return fmt.Errorf("scaffold is not installed: %w\n\nPlease install scaffold from https://github.com/hay-kot/scaffold", err)
	}

	var options *InitOptions
	var err error
	
	// For testing: use provided options instead of prompting
	if ic.testOptions != nil {
		options = ic.testOptions
	} else {
		options, err = ic.promptInitOptions(opts...)
		if err != nil {
			return fmt.Errorf("failed to get init options: %w", err)
		}
	}

	templatesDir, err := ic.ensureTemplatesExtracted()
	if err != nil {
		return fmt.Errorf("failed to extract templates: %w", err)
	}

	templatePath := filepath.Join(templatesDir, options.Template)
	if err := ic.scaffold.Run(options.ProjectName, templatePath); err != nil {
		return fmt.Errorf("failed to scaffold project: %w", err)
	}

	fmt.Printf("✅ Successfully created %s project: %s\n", options.Template, options.ProjectName)
	return nil
}

func (ic *InitCommand) promptInitOptions(opts ...tea.ProgramOption) (*InitOptions, error) {
	var projectName string
	var template string

	form := ic.createInitForm(&projectName, &template)
	
	if len(opts) > 0 {
		// For testing: run with provided options
		program := tea.NewProgram(form, opts...)
		if _, err := program.Run(); err != nil {
			return nil, err
		}
	} else {
		// Normal execution
		if err := form.Run(); err != nil {
			return nil, err
		}
	}

	return &InitOptions{
		ProjectName: projectName,
		Template:    template,
	}, nil
}

func (ic *InitCommand) createInitForm(projectName *string, template *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Name of your new OKRA project").
				Value(projectName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("project name cannot be empty")
					}
					if _, err := ic.filesystem.Stat(s); err == nil {
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
				Value(template),
		),
	)
}

func (ic *InitCommand) ensureTemplatesExtracted() (string, error) {
	homeDir, err := ic.filesystem.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	okraDir := filepath.Join(homeDir, ".okra", "templates")
	
	if _, err := ic.filesystem.Stat(okraDir); err == nil {
		return okraDir, nil
	}

	if err := ic.filesystem.MkdirAll(okraDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create templates directory: %w", err)
	}

	if err := ic.extractTemplates(okraDir); err != nil {
		return "", fmt.Errorf("failed to extract templates: %w", err)
	}

	return okraDir, nil
}

func (ic *InitCommand) extractTemplates(destDir string) error {
	return fs.WalkDir(ic.templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
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
			return ic.filesystem.MkdirAll(destPath, 0755)
		}

		data, err := fs.ReadFile(ic.templatesFS, path)
		if err != nil {
			return err
		}

		return ic.filesystem.WriteFile(destPath, data, 0644)
	})
}