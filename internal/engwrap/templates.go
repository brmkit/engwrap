package engwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListTemplates lists all available templates in a table format
func ListTemplates() error {
	templatesDir, err := GetEngwrapTemplatesDir()
	if err != nil {
		return err
	}

	// ensure directory exists
	if err := InitializeEngwrapDirs(); err != nil {
		return err
	}

	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return fmt.Errorf("error reading templates directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No templates found.")
		fmt.Printf("Add templates to: %s\n", templatesDir)
		return nil
	}

	// collect template data
	type templateInfo struct {
		name        string
		image       string
		defaultName string
	}

	var templates []templateInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// only show .yaml and .yml files
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		// remove .yaml/.yml extension for display
		templateName := strings.TrimSuffix(name, ".yaml")
		templateName = strings.TrimSuffix(templateName, ".yml")

		templatePath := filepath.Join(templatesDir, entry.Name())

		info := templateInfo{
			name:        templateName,
			image:       "unknown",
			defaultName: "-",
		}

		// try to load and get info
		config, err := LoadConfig(templatePath)
		if err == nil {
			info.image = config.Image
			if config.Name != "" {
				info.defaultName = config.Name
			}
		}

		templates = append(templates, info)
	}

	if len(templates) == 0 {
		fmt.Println("No templates found.")
		fmt.Printf("Add templates to: %s\n", templatesDir)
		return nil
	}

	// print table header
	fmt.Printf("%-20s %-40s %-20s\n", "TEMPLATE", "IMAGE", "DEFAULT NAME")
	fmt.Println(strings.Repeat("-", 82))

	// print table rows
	for _, t := range templates {
		fmt.Printf("%-20s %-40s %-20s\n", t.name, t.image, t.defaultName)
	}

	return nil
}

// AddTemplate copies a template file to the templates directory
func AddTemplate(srcPath string) error {
	// check if source file exists
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("error accessing source file: %w", err)
	}

	// read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("error reading source file: %w", err)
	}

	// get templates directory
	templatesDir, err := GetEngwrapTemplatesDir()
	if err != nil {
		return err
	}

	// ensure directory exists
	if err := InitializeEngwrapDirs(); err != nil {
		return err
	}

	// destination path
	filename := filepath.Base(srcPath)
	destPath := filepath.Join(templatesDir, filename)

	// write file
	if err := os.WriteFile(destPath, content, 0600); err != nil {
		return fmt.Errorf("error writing template file: %w", err)
	}

	fmt.Printf("Template '%s' added successfully.\n", filename)
	return nil
}
