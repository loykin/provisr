package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loykin/provisr/pkg/template"
)

// getTemplatesDirectory returns the templates directory path
func (c *command) getTemplatesDirectory() string {
	return "templates"
}

// TemplateCreate creates a new process template
func (c *command) TemplateCreate(f TemplateCreateFlags) error {
	// Use provided name or default based on type
	templateName := f.Name
	if templateName == "" {
		templateName = f.Type + "-sample"
	}

	// Determine output file path
	outputPath := f.Output
	if outputPath == "" {
		templatesDir := c.getTemplatesDirectory()
		if err := os.MkdirAll(templatesDir, 0o755); err != nil {
			return fmt.Errorf("failed to create templates directory: %w", err)
		}
		outputPath = filepath.Join(templatesDir, templateName+".json")
	}

	// Check if file already exists and force flag not set
	if _, err := os.Stat(outputPath); err == nil && !f.Force {
		return fmt.Errorf("template file '%s' already exists (use --force to overwrite)", outputPath)
	}

	// Generate template content based on type
	generator := template.NewGenerator()
	templateContent, err := generator.GenerateJSON(template.TemplateType(f.Type), templateName)
	if err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	// Write template file
	if err := os.WriteFile(outputPath, templateContent, 0o644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}

	fmt.Printf("Template '%s' created: %s\n", templateName, outputPath)
	fmt.Printf("Edit the template and register with: provisr register-file %s\n", outputPath)
	return nil
}
