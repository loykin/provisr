package template

import (
	"encoding/json"
	"fmt"
)

// TemplateType represents the type of template to generate
type TemplateType string

const (
	TypeWeb        TemplateType = "web"
	TypeWebapp     TemplateType = "webapp"
	TypeAPI        TemplateType = "api"
	TypeService    TemplateType = "service"
	TypeWorker     TemplateType = "worker"
	TypeBackground TemplateType = "background"
	TypeDatabase   TemplateType = "database"
	TypeDB         TemplateType = "db"
	TypeCron       TemplateType = "cron"
	TypeScheduled  TemplateType = "scheduled"
	TypeSimple     TemplateType = "simple"
	TypeBasic      TemplateType = "basic"
)

// ProcessTemplate represents a process configuration template
type ProcessTemplate struct {
	Name        string                 `json:"name"`
	Command     string                 `json:"command"`
	WorkDir     string                 `json:"work_dir,omitempty"`
	AutoRestart *bool                  `json:"auto_restart,omitempty"`
	Priority    *int                   `json:"priority,omitempty"`
	Log         *LogConfig             `json:"log,omitempty"`
	Env         []string               `json:"env,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	File *FileLogConfig `json:"file,omitempty"`
}

// FileLogConfig represents file logging configuration
type FileLogConfig struct {
	Dir string `json:"dir"`
}

// Generator provides template generation functionality
type Generator struct{}

// NewGenerator creates a new template generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a process template based on the specified type and name
func (g *Generator) Generate(templateType TemplateType, name string) (*ProcessTemplate, error) {
	switch templateType {
	case TypeWeb, TypeWebapp:
		return g.generateWebTemplate(name), nil
	case TypeAPI, TypeService:
		return g.generateAPITemplate(name), nil
	case TypeWorker, TypeBackground:
		return g.generateWorkerTemplate(name), nil
	case TypeDatabase, TypeDB:
		return g.generateDatabaseTemplate(name), nil
	case TypeCron, TypeScheduled:
		return g.generateCronTemplate(name), nil
	case TypeSimple, TypeBasic:
		return g.generateSimpleTemplate(name), nil
	default:
		return nil, fmt.Errorf("unknown template type: %s (supported: web, api, worker, database, cron, simple)", templateType)
	}
}

// GenerateJSON creates a JSON representation of the template
func (g *Generator) GenerateJSON(templateType TemplateType, name string) ([]byte, error) {
	template, err := g.Generate(templateType, name)
	if err != nil {
		return nil, err
	}

	// Convert to map for JSON serialization to handle omitempty properly
	templateMap := g.templateToMap(template)

	jsonData, err := json.MarshalIndent(templateMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template: %w", err)
	}

	return jsonData, nil
}

// GetSupportedTypes returns a list of all supported template types
func (g *Generator) GetSupportedTypes() []string {
	return []string{
		string(TypeWeb),
		string(TypeAPI),
		string(TypeWorker),
		string(TypeDatabase),
		string(TypeCron),
		string(TypeSimple),
	}
}

// templateToMap converts a ProcessTemplate to a map for JSON serialization
func (g *Generator) templateToMap(template *ProcessTemplate) map[string]interface{} {
	result := map[string]interface{}{
		"name":    template.Name,
		"command": template.Command,
	}

	if template.WorkDir != "" {
		result["work_dir"] = template.WorkDir
	}

	if template.AutoRestart != nil {
		result["auto_restart"] = *template.AutoRestart
	}

	if template.Priority != nil {
		result["priority"] = *template.Priority
	}

	if template.Log != nil {
		result["log"] = map[string]interface{}{
			"file": map[string]interface{}{
				"dir": template.Log.File.Dir,
			},
		}
	}

	if len(template.Env) > 0 {
		result["env"] = template.Env
	}

	// Add any extra fields
	for key, value := range template.Extra {
		result[key] = value
	}

	return result
}

// Helper functions to create specific templates

func (g *Generator) generateWebTemplate(name string) *ProcessTemplate {
	autoRestart := true
	return &ProcessTemplate{
		Name:        name,
		Command:     "python -m http.server 8000",
		WorkDir:     "/app",
		AutoRestart: &autoRestart,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/" + name,
			},
		},
		Env: []string{
			"PORT=8000",
			"ENV=production",
		},
	}
}

func (g *Generator) generateAPITemplate(name string) *ProcessTemplate {
	autoRestart := true
	priority := 10
	return &ProcessTemplate{
		Name:        name,
		Command:     "./api-server",
		WorkDir:     "/app",
		AutoRestart: &autoRestart,
		Priority:    &priority,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/" + name,
			},
		},
		Env: []string{
			"PORT=3000",
			"LOG_LEVEL=info",
		},
	}
}

func (g *Generator) generateWorkerTemplate(name string) *ProcessTemplate {
	autoRestart := true
	priority := 20
	return &ProcessTemplate{
		Name:        name,
		Command:     "./worker",
		WorkDir:     "/app",
		AutoRestart: &autoRestart,
		Priority:    &priority,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/" + name,
			},
		},
		Env: []string{
			"WORKER_THREADS=4",
			"LOG_LEVEL=info",
		},
	}
}

func (g *Generator) generateDatabaseTemplate(name string) *ProcessTemplate {
	autoRestart := true
	priority := 5
	return &ProcessTemplate{
		Name:        name,
		Command:     "mongod --dbpath /data/db --port 27017",
		WorkDir:     "/data",
		AutoRestart: &autoRestart,
		Priority:    &priority,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/" + name,
			},
		},
		Env: []string{
			"DB_PORT=27017",
			"DB_PATH=/data/db",
		},
	}
}

func (g *Generator) generateCronTemplate(name string) *ProcessTemplate {
	autoRestart := false
	priority := 30
	return &ProcessTemplate{
		Name:        name,
		Command:     "./scheduled-task",
		WorkDir:     "/app",
		AutoRestart: &autoRestart,
		Priority:    &priority,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/" + name,
			},
		},
		Env: []string{
			"SCHEDULE=daily",
			"LOG_LEVEL=info",
		},
	}
}

func (g *Generator) generateSimpleTemplate(name string) *ProcessTemplate {
	return &ProcessTemplate{
		Name:    name,
		Command: "echo 'Hello from " + name + "'",
	}
}
