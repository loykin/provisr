package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	cronpkg "github.com/loykin/provisr/internal/cron"
	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/process_group"
)

type Config struct {
	UseOSEnv          bool           `mapstructure:"use_os_env"`
	EnvFiles          []string       `mapstructure:"env_files"`
	Env               []string       `mapstructure:"env"`
	ProgramsDirectory string         `mapstructure:"programs_directory"`
	Groups            []GroupConfig  `mapstructure:"groups"`
	Store             *StoreConfig   `mapstructure:"store"`
	History           *HistoryConfig `mapstructure:"history"`
	Metrics           *MetricsConfig `mapstructure:"metrics"`
	Log               *LogConfig     `mapstructure:"log"`
	Server            *ServerConfig  `mapstructure:"server"`

	// Inline processes parsed as discriminated union entries
	Processes []ProcessConfig `mapstructure:"processes"`

	// Computed/aggregated fields
	GlobalEnv  []string
	Specs      []process.Spec
	GroupSpecs []process_group.GroupSpec
	CronJobs   []*cronpkg.Job

	configPath string
}

type GroupConfig struct {
	Name    string   `mapstructure:"name"`
	Members []string `mapstructure:"members"`
}

type StoreConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	DSN     string `mapstructure:"dsn"`
}

type HistoryConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	InStore         *bool  `mapstructure:"in_store"`
	OpenSearchURL   string `mapstructure:"opensearch_url"`
	OpenSearchIndex string `mapstructure:"opensearch_index"`
	ClickHouseURL   string `mapstructure:"clickhouse_url"`
	ClickHouseTable string `mapstructure:"clickhouse_table"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Listen  string `mapstructure:"listen"`
}

type LogConfig struct {
	Dir        string `mapstructure:"dir"`
	Stdout     string `mapstructure:"stdout"`
	Stderr     string `mapstructure:"stderr"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

type ServerConfig struct {
	Listen   string `mapstructure:"listen"`
	BasePath string `mapstructure:"base_path"`
}

type ProcessConfig struct {
	Type string         `mapstructure:"type"` // process, cronjob
	Spec map[string]any `mapstructure:"spec"` // specific config
}

// helper to decode map[string]any to a target type using mapstructure
func decodeTo[T any](m map[string]any) (T, error) {
	var out T
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		Result:           &out,
	})
	if err != nil {
		return out, err
	}
	if err := dec.Decode(m); err != nil {
		return out, err
	}
	return out, nil
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{configPath: configPath}

	if err := parseConfigFile(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize aggregated fields
	config.Specs = make([]process.Spec, 0)
	config.CronJobs = []*cronpkg.Job{}

	// 1) Inline processes: discriminated union decoding
	for _, pc := range config.Processes {
		switch strings.ToLower(strings.TrimSpace(pc.Type)) {
		case "", "process":
			// default to process if type is empty
			spec, err := decodeTo[process.Spec](pc.Spec)
			if err != nil {
				return nil, fmt.Errorf("decode process spec: %w", err)
			}
			if strings.TrimSpace(spec.Name) == "" {
				return nil, fmt.Errorf("process requires name")
			}
			if strings.TrimSpace(spec.Command) == "" {
				return nil, fmt.Errorf("process %q requires command", spec.Name)
			}
			if err := convertDetectorConfigs(&spec); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for process %s: %w", spec.Name, err)
			}
			config.Specs = append(config.Specs, spec)
		case "cron", "cronjob":
			job, err := decodeTo[cronpkg.Job](pc.Spec)
			if err != nil {
				return nil, fmt.Errorf("decode cronjob spec: %w", err)
			}
			// Allow job.Name to be omitted if Spec.Name is present
			if strings.TrimSpace(job.Name) == "" {
				job.Name = strings.TrimSpace(job.Spec.Name)
			}
			if strings.TrimSpace(job.Name) == "" {
				return nil, fmt.Errorf("cronjob requires name")
			}
			if strings.TrimSpace(job.Spec.Command) == "" {
				return nil, fmt.Errorf("cronjob %q requires command", job.Name)
			}
			if strings.TrimSpace(job.Schedule) == "" {
				return nil, fmt.Errorf("cronjob %q requires schedule", job.Name)
			}
			if err := convertDetectorConfigs(&job.Spec); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for cronjob %s: %w", job.Name, err)
			}
			config.Specs = append(config.Specs, job.Spec)
			config.CronJobs = append(config.CronJobs, &job)
		default:
			return nil, fmt.Errorf("unknown process type %q (allowed: process, cronjob)", pc.Type)
		}
	}

	// 2) Programs directory - use config setting or default to "programs"
	var programsDir string
	if config.ProgramsDirectory != "" {
		if filepath.IsAbs(config.ProgramsDirectory) {
			programsDir = config.ProgramsDirectory
		} else {
			programsDir = filepath.Join(filepath.Dir(configPath), config.ProgramsDirectory)
		}
	} else {
		// Default: "programs" directory next to the main config file
		programsDir = filepath.Join(filepath.Dir(configPath), "programs")
	}

	if specs, err := loadProgramSpecs(programsDir); err != nil {
		return nil, fmt.Errorf("failed to load programs from %s: %w", programsDir, err)
	} else if len(specs) > 0 {
		// convert detectors per program spec for consistency
		for i := range specs {
			if err := convertDetectorConfigs(&specs[i]); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for program %s: %w", specs[i].Name, err)
			}
		}
		config.Specs = append(config.Specs, specs...)
	}

	// Compute Global Env after merging
	globalEnv, err := computeGlobalEnv(config.UseOSEnv, config.EnvFiles, config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to compute global env: %w", err)
	}
	config.GlobalEnv = globalEnv

	// Build groups using the aggregated specs
	groupSpecs, err := buildGroups(config.Groups, config.Specs)
	if err != nil {
		return nil, fmt.Errorf("failed to build groups: %w", err)
	}
	config.GroupSpecs = groupSpecs

	return config, nil
}

func parseConfigFile(configPath string, out interface{}) error {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.Unmarshal(out); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// loadProgramSpecs loads process.Spec files from a directory named "programs" next to the
// main configuration file. It supports toml, yaml/yml, and json formats via viper.
func loadProgramSpecs(programsDir string) ([]process.Spec, error) {
	infos, err := os.ReadDir(programsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Supported file extensions
	exts := []string{".toml", ".yaml", ".yml", ".json"}
	supported := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		supported[e] = struct{}{}
	}

	var specs []process.Spec
	for _, de := range infos {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, ".") { // skip hidden files
			continue
		}
		full := filepath.Join(programsDir, name)
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := supported[ext]; !ok {
			continue // unsupported file
		}

		v := viper.New()
		v.SetConfigFile(full)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read %s: %w", full, err)
		}
		var s process.Spec
		if err := v.Unmarshal(&s); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", full, err)
		}
		if strings.TrimSpace(s.Name) == "" {
			return nil, fmt.Errorf("program file %s missing required 'name'", full)
		}
		if strings.TrimSpace(s.Command) == "" {
			return nil, fmt.Errorf("program file %s missing required 'command'", full)
		}
		specs = append(specs, s)
	}
	return specs, nil
}

func computeGlobalEnv(useOSEnv bool, envFiles []string, env []string) ([]string, error) {
	envMap := make(map[string]string)

	if useOSEnv {
		for _, kv := range os.Environ() {
			if i := strings.IndexByte(kv, '='); i >= 0 {
				envMap[kv[:i]] = kv[i+1:]
			}
		}
	}

	for _, envFile := range envFiles {
		fileEnv, err := loadEnvFile(envFile)
		if err != nil {
			return nil, err
		}
		for key, value := range fileEnv {
			envMap[key] = value
		}
	}

	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			envMap[kv[:i]] = kv[i+1:]
		}
	}

	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	sort.Strings(result)

	return result, nil
}

func buildGroups(groupConfigs []GroupConfig, specs []process.Spec) ([]process_group.GroupSpec, error) {
	specMap := make(map[string]process.Spec, len(specs))
	for _, spec := range specs {
		specMap[spec.Name] = spec
	}

	groups := make([]process_group.GroupSpec, 0, len(groupConfigs))
	for _, gc := range groupConfigs {
		if gc.Name == "" {
			return nil, fmt.Errorf("group requires name")
		}
		if len(gc.Members) == 0 {
			return nil, fmt.Errorf("group %s requires members", gc.Name)
		}

		memberSpecs := make([]process.Spec, 0, len(gc.Members))
		for _, memberName := range gc.Members {
			spec, exists := specMap[memberName]
			if !exists {
				return nil, fmt.Errorf("group %s references unknown member %s", gc.Name, memberName)
			}
			memberSpecs = append(memberSpecs, spec)
		}

		groups = append(groups, process_group.GroupSpec{
			Name:    gc.Name,
			Members: memberSpecs,
		})
	}

	return groups, nil
}

func loadEnvFile(filePath string) (map[string]string, error) {
	// #nosec 304
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	env := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.IndexByte(line, '='); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			env[key] = value
		} else {
			return nil, fmt.Errorf("invalid env line at %s:%d: %s", filePath, i+1, line)
		}
	}

	return env, nil
}

// convertDetectorConfigs converts DetectorConfig slice to actual Detector interfaces
func convertDetectorConfigs(spec *process.Spec) error {
	if len(spec.DetectorConfigs) == 0 {
		return nil
	}

	spec.Detectors = make([]detector.Detector, len(spec.DetectorConfigs))
	for i, config := range spec.DetectorConfigs {
		switch config.Type {
		case "pidfile":
			if config.Path == "" {
				return fmt.Errorf("pidfile detector requires 'path' field")
			}
			spec.Detectors[i] = &detector.PIDFileDetector{PIDFile: config.Path}
		case "command":
			if config.Command == "" {
				return fmt.Errorf("command detector requires 'command' field")
			}
			spec.Detectors[i] = &detector.CommandDetector{Command: config.Command}
		default:
			return fmt.Errorf("unknown detector type: %s", config.Type)
		}
	}

	return nil
}
