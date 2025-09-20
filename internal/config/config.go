package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"

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

	// Inline process specs within the main config file (e.g., [[processes]] blocks)
	Processes []process.Spec `mapstructure:"processes"`

	// Computed/aggregated fields
	GlobalEnv  []string
	Specs      []process.Spec
	GroupSpecs []process_group.GroupSpec
	CronJobs   []CronJob

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

// CronJob represents a cron job configuration
type CronJob struct {
	Name      string       `json:"name"`
	Spec      process.Spec `json:"spec"`
	Schedule  string       `json:"schedule"`
	Singleton bool         `json:"singleton"`
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{configPath: configPath}

	if err := parseConfigFile(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize aggregated fields
	config.Specs = make([]process.Spec, 0)
	config.CronJobs = []CronJob{}

	// 1) Inline processes from [[processes]] blocks
	if len(config.Processes) > 0 {
		config.Specs = append(config.Specs, config.Processes...)
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
		config.Specs = append(config.Specs, specs...)
	}

	// Also extract cron jobs from inline processes if present
	if jobs, err := loadCronJobsFromConfig(configPath); err != nil {
		return nil, fmt.Errorf("failed to load cron jobs: %w", err)
	} else if len(jobs) > 0 {
		config.CronJobs = append(config.CronJobs, jobs...)
	}

	// Convert DetectorConfigs to actual Detector interfaces
	for i := range config.Specs {
		if err := convertDetectorConfigs(&config.Specs[i]); err != nil {
			return nil, fmt.Errorf("failed to convert detectors for process %s: %w", config.Specs[i].Name, err)
		}
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

// loadCronJobsFromConfig parses the main config file and extracts any processes that
// have a 'schedule' (and optional 'singleton') field, returning them as CronJobs.
func loadCronJobsFromConfig(configPath string) ([]CronJob, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config for cron jobs: %w", err)
	}
	// Define a lightweight view of processes that includes schedule fields
	type procRaw struct {
		process.Spec `mapstructure:",squash"`
		Schedule     string `mapstructure:"schedule"`
		Singleton    bool   `mapstructure:"singleton"`
	}
	var raw struct {
		Processes []procRaw `mapstructure:"processes"`
	}
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cron jobs: %w", err)
	}
	jobs := make([]CronJob, 0)
	for _, p := range raw.Processes {
		if strings.TrimSpace(p.Schedule) == "" {
			continue
		}
		jobs = append(jobs, CronJob{
			Name:      p.Name,
			Spec:      p.Spec,
			Schedule:  p.Schedule,
			Singleton: p.Singleton,
		})
	}
	return jobs, nil
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
