package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-viper/mapstructure/v2"
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

// decodeProcessEntry decodes and validates a ProcessConfig entry (process or cronjob).
// ctx is used to improve error messages with the source (e.g., filename or "inline processes").
func decodeProcessEntry(pc ProcessConfig, ctx string) (process.Spec, *cronpkg.Job, error) {
	var zero process.Spec
	typ := strings.ToLower(strings.TrimSpace(pc.Type))
	switch typ {
	case "", "process":
		sp, err := decodeTo[process.Spec](pc.Spec)
		if err != nil {
			return zero, nil, fmt.Errorf("decode process spec in %s: %w", ctx, err)
		}
		if strings.TrimSpace(sp.Name) == "" {
			return zero, nil, fmt.Errorf("%s: process requires name", ctx)
		}
		if strings.TrimSpace(sp.Command) == "" {
			return zero, nil, fmt.Errorf("%s: process %q requires command", ctx, sp.Name)
		}
		return sp, nil, nil
	case "cron", "cronjob":
		jb, err := decodeTo[cronpkg.Job](pc.Spec)
		if err != nil {
			return zero, nil, fmt.Errorf("decode cronjob spec in %s: %w", ctx, err)
		}
		if strings.TrimSpace(jb.Name) == "" {
			jb.Name = strings.TrimSpace(jb.Spec.Name)
		}
		if strings.TrimSpace(jb.Name) == "" {
			return zero, nil, fmt.Errorf("%s: cronjob requires name", ctx)
		}
		if strings.TrimSpace(jb.Spec.Command) == "" {
			return zero, nil, fmt.Errorf("%s: cronjob %q requires command", ctx, jb.Name)
		}
		if strings.TrimSpace(jb.Schedule) == "" {
			return zero, nil, fmt.Errorf("%s: cronjob %q requires schedule", ctx, jb.Name)
		}
		return jb.Spec, &jb, nil
	default:
		return zero, nil, fmt.Errorf("%s: unknown process type %q (allowed: process, cronjob)", ctx, pc.Type)
	}
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{configPath: configPath}

	if err := parseConfigFile(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize aggregated fields
	config.Specs = make([]process.Spec, 0)
	config.CronJobs = []*cronpkg.Job{}

	// 1) Inline processes: discriminated union decoding (refactored)
	for _, pc := range config.Processes {
		spec, job, err := decodeProcessEntry(pc, "inline processes")
		if err != nil {
			return nil, err
		}
		// convert detectors after decode
		if err := convertDetectorConfigs(&spec); err != nil {
			return nil, fmt.Errorf("failed to convert detectors for process %s: %w", spec.Name, err)
		}
		config.Specs = append(config.Specs, spec)
		if job != nil {
			if err := convertDetectorConfigs(&job.Spec); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for cronjob %s: %w", job.Name, err)
			}
			config.CronJobs = append(config.CronJobs, job)
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

	if specs, jobs, err := loadProgramEntries(programsDir); err != nil {
		return nil, fmt.Errorf("failed to load programs from %s: %w", programsDir, err)
	} else {
		// convert detectors per program spec for consistency
		for i := range specs {
			if err := convertDetectorConfigs(&specs[i]); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for program %s: %w", specs[i].Name, err)
			}
		}
		for _, j := range jobs {
			if err := convertDetectorConfigs(&j.Spec); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for cronjob %s: %w", j.Name, err)
			}
		}
		config.Specs = append(config.Specs, specs...)
		config.CronJobs = append(config.CronJobs, jobs...)
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
	// Apply global log defaults (dir/stdout/stderr and rotation limits) to all specs
	if err := applyGlobalLogDefaults(config); err != nil {
		return nil, fmt.Errorf("failed to apply global log defaults: %w", err)
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

// loadProgramEntries loads program entries from the programs directory using the same
// discriminated-union format as inline [[processes]] blocks: {type, spec}.
// Supported file extensions: toml, yaml/yml, json. No legacy plain process.Spec files supported.
func loadProgramEntries(programsDir string) ([]process.Spec, []*cronpkg.Job, error) {
	infos, err := os.ReadDir(programsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	// Supported file extensions
	exts := []string{".toml", ".yaml", ".yml", ".json"}
	supported := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		supported[e] = struct{}{}
	}

	var specs []process.Spec
	var jobs []*cronpkg.Job
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
			return nil, nil, fmt.Errorf("read %s: %w", full, err)
		}

		var pc ProcessConfig
		if err := v.Unmarshal(&pc); err != nil {
			return nil, nil, fmt.Errorf("unmarshal %s: %w", full, err)
		}

		sp, jb, err := decodeProcessEntry(pc, full)
		if err != nil {
			return nil, nil, err
		}
		specs = append(specs, sp)
		if jb != nil {
			jobs = append(jobs, jb)
		}
	}
	return specs, jobs, nil
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

func applyGlobalLogDefaults(cfg *Config) error {
	if cfg.Log == nil {
		return nil
	}
	// Resolve global paths relative to the main config file directory
	baseDir := filepath.Dir(cfg.configPath)
	makeAbs := func(p string) string {
		if p == "" {
			return ""
		}
		if filepath.IsAbs(p) {
			return filepath.Clean(p)
		}
		return filepath.Clean(filepath.Join(baseDir, p))
	}

	globalDir := makeAbs(cfg.Log.Dir)
	globalStdout := makeAbs(cfg.Log.Stdout)
	globalStderr := makeAbs(cfg.Log.Stderr)

	apply := func(sp *process.Spec) {
		// Only set path fields when the spec hasn't set any of them
		noPathsSet := sp.Log.File.Dir == "" && sp.Log.File.StdoutPath == "" && sp.Log.File.StderrPath == ""
		if noPathsSet {
			if globalStdout != "" {
				sp.Log.File.StdoutPath = globalStdout
			}
			if globalStderr != "" {
				sp.Log.File.StderrPath = globalStderr
			}
			if sp.Log.File.StdoutPath == "" && sp.Log.File.StderrPath == "" {
				// Fall back to directory-based naming if explicit files not provided
				sp.Log.File.Dir = globalDir
			}
		}
		// Apply rotation defaults if zero
		if sp.Log.File.MaxSizeMB == 0 && cfg.Log.MaxSizeMB > 0 {
			sp.Log.File.MaxSizeMB = cfg.Log.MaxSizeMB
		}
		if sp.Log.File.MaxBackups == 0 && cfg.Log.MaxBackups > 0 {
			sp.Log.File.MaxBackups = cfg.Log.MaxBackups
		}
		if sp.Log.File.MaxAgeDays == 0 && cfg.Log.MaxAgeDays > 0 {
			sp.Log.File.MaxAgeDays = cfg.Log.MaxAgeDays
		}
		// Compress default copies boolean as-is only when any path configured
		if noPathsSet {
			// If we just set paths above, respect global Compress
			sp.Log.File.Compress = cfg.Log.Compress
		}
	}

	for i := range cfg.Specs {
		apply(&cfg.Specs[i])
	}
	for _, j := range cfg.CronJobs {
		apply(&j.Spec)
	}
	return nil
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
