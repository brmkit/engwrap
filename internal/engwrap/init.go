package engwrap

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	engwrapDir   = ".engwrap"
	workDir      = "work"
	templatesDir = "templates"
)

// InitializeEngwrapDirs creates the necessary directories in $HOME/.engwrap
func InitializeEngwrapDirs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error getting home directory: %w", err)
	}

	engwrapPath := filepath.Join(homeDir, engwrapDir)
	workPath := filepath.Join(engwrapPath, workDir)
	templatesPath := filepath.Join(engwrapPath, templatesDir)

	// create directories
	if err := os.MkdirAll(workPath, 0700); err != nil {
		return fmt.Errorf("error creating work directory: %w", err)
	}

	if err := os.MkdirAll(templatesPath, 0700); err != nil {
		return fmt.Errorf("error creating templates directory: %w", err)
	}

	// create base.yml example template if templates directory is empty
	entries, err := os.ReadDir(templatesPath)
	if err == nil && len(entries) == 0 {
		baseTemplatePath := filepath.Join(templatesPath, "base.yml")
		if err := createBaseTemplate(baseTemplatePath); err != nil {
			// non-fatal error, just log it
			fmt.Printf("Warning: could not create base template: %v\n", err)
		}
	}

	return nil
}

// createBaseTemplate creates a base.yml example template file
func createBaseTemplate(path string) error {
	baseTemplate := `name: base-env
image: debian:bookworm-slim
init_commands:
  - apt-get update
  - apt-get install -y bash curl wget bsdutils
bashrc_customs:
  - export PS1="($(date +%d/%m/%Y\ -\ %H:%M)) ${ENV_NAME} \w> "
`

	return os.WriteFile(path, []byte(baseTemplate), 0600)
}

// GetEngwrapWorkDir returns the path to $HOME/.engwrap/work
func GetEngwrapWorkDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %w", err)
	}
	return filepath.Join(homeDir, engwrapDir, workDir), nil
}

// GetEngwrapTemplatesDir returns the path to $HOME/.engwrap/templates
func GetEngwrapTemplatesDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error getting home directory: %w", err)
	}
	return filepath.Join(homeDir, engwrapDir, templatesDir), nil
}

// GetTemplatePath returns the full path to a template file
func GetTemplatePath(templateName string) (string, error) {
	templatesDir, err := GetEngwrapTemplatesDir()
	if err != nil {
		return "", err
	}

	// template name is the filename without extension
	templatePath := filepath.Join(templatesDir, templateName)

	// check if it exists as-is
	if _, err := os.Stat(templatePath); err == nil {
		return templatePath, nil
	}

	// check with .yaml and .yml extension
	for _, ext := range []string{".yaml", ".yml"} {
		path := templatePath + ext
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("template '%s' not found", templateName)
}
