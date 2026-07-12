package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"github.com/loykin/provisr/core"
)

type Config struct {
	UseOSEnv          bool            `mapstructure:"use_os_env"`
	EnvFiles          []string        `mapstructure:"env_files"`
	Env               []string        `mapstructure:"env"`
	ProgramsDirectory string          `mapstructure:"programs_directory"`
	PIDDir            string          `mapstructure:"pid_dir"`
	Groups            []GroupConfig   `mapstructure:"groups"`
	History           *HistoryConfig  `mapstructure:"history"`
	Metrics           *MetricsConfig  `mapstructure:"metrics"`
	Log               *core.LogConfig `mapstructure:"log"`
	Daemon            *DaemonConfig   `mapstructure:"daemon"`
	Server            *ServerConfig   `mapstructure:"server"`

	// Inline processes parsed as discriminated union entries
	Processes []ProcessConfig `mapstructure:"processes"`
}

type LoadedConfig struct {
	Config
	GlobalEnv                 []string
	Specs                     []core.Spec
	GroupSpecs                []core.ServiceGroup
	CronJobs                  []core.CronJob
	ResolvedProgramsDirectory string

	configPath string
}

type GroupConfig struct {
	Name    string   `mapstructure:"name"`
	Members []string `mapstructure:"members"`
}

type HistoryConfig struct {
	Enabled bool                `mapstructure:"enabled"`
	Primary string              `mapstructure:"primary"`
	Stores  HistoryStoresConfig `mapstructure:"stores"`
}

type HistoryStoresConfig struct {
	SQLite     *SQLiteHistoryStoreConfig     `mapstructure:"sqlite"`
	Postgres   *PostgresHistoryStoreConfig   `mapstructure:"postgres"`
	ClickHouse *ClickHouseHistoryStoreConfig `mapstructure:"clickhouse"`
	OpenSearch *OpenSearchHistoryStoreConfig `mapstructure:"opensearch"`
}

type SQLHistoryStoreConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	DSN             string        `mapstructure:"dsn"`
	Migrate         *bool         `mapstructure:"migrate"`
	Retention       time.Duration `mapstructure:"retention"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type SQLiteHistoryStoreConfig struct {
	SQLHistoryStoreConfig `mapstructure:",squash"`
}

type PostgresHistoryStoreConfig struct {
	SQLHistoryStoreConfig `mapstructure:",squash"`
}

type ClickHouseHistoryStoreConfig struct {
	SQLHistoryStoreConfig `mapstructure:",squash"`
	Table                 string `mapstructure:"table"`
}

type OpenSearchHistoryStoreConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	URL             string        `mapstructure:"url"`
	Index           string        `mapstructure:"index"`
	Migrate         *bool         `mapstructure:"migrate"`
	Retention       time.Duration `mapstructure:"retention"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type MetricsConfig struct {
	Enabled        bool                  `mapstructure:"enabled"`
	Listen         string                `mapstructure:"listen"`
	ProcessMetrics *ProcessMetricsConfig `mapstructure:"process_metrics"`
}

type ProcessMetricsConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Interval   time.Duration `mapstructure:"interval"`
	MaxHistory int           `mapstructure:"max_history"`
}

type DaemonConfig struct {
	PIDFile string `mapstructure:"pid_file"`
	LogFile string `mapstructure:"log_file"`
}

type ServerConfig struct {
	Listen   string      `mapstructure:"listen"`
	BasePath string      `mapstructure:"base_path"`
	TLS      *TLSConfig  `mapstructure:"tls"`
	Auth     *AuthConfig `mapstructure:"auth"`
}

type TLSConfig struct {
	Enabled      bool        `mapstructure:"enabled"`
	MinVersion   string      `mapstructure:"min_version"`
	MaxVersion   string      `mapstructure:"max_version"`
	CertFile     string      `mapstructure:"cert_file"`
	KeyFile      string      `mapstructure:"key_file"`
	Dir          string      `mapstructure:"dir"`
	AutoGenerate bool        `mapstructure:"auto_generate"`
	AutoGen      *AutoGenTLS `mapstructure:"auto_gen"`
}

type AutoGenTLS struct {
	CommonName   string   `mapstructure:"common_name"`
	Organization string   `mapstructure:"organization"`
	DNSNames     []string `mapstructure:"dns_names"`
	IPAddresses  []string `mapstructure:"ip_addresses"`
	ValidDays    int      `mapstructure:"valid_days"`
}

type AuthConfig struct {
	Enabled    bool            `mapstructure:"enabled"`
	Store      AuthStoreConfig `mapstructure:"store"`
	JWTSecret  string          `mapstructure:"jwt_secret"`
	TokenTTL   time.Duration   `mapstructure:"token_ttl"`
	BcryptCost int             `mapstructure:"bcrypt_cost"`
}

type AuthStoreConfig struct {
	Type         string `mapstructure:"type"` // "sqlite" or "postgresql"
	Migrate      *bool  `mapstructure:"migrate"`
	Path         string `mapstructure:"path,omitempty"`
	Host         string `mapstructure:"host,omitempty"`
	Port         int    `mapstructure:"port,omitempty"`
	Database     string `mapstructure:"database,omitempty"`
	Username     string `mapstructure:"username,omitempty"`
	Password     string `mapstructure:"password,omitempty"`
	SSLMode      string `mapstructure:"ssl_mode,omitempty"`
	MaxOpenConns int    `mapstructure:"max_open_conns,omitempty"`
	MaxIdleConns int    `mapstructure:"max_idle_conns,omitempty"`
}

type ProcessConfig struct {
	Type string         `mapstructure:"type"` // process, cronjob
	Spec map[string]any `mapstructure:"spec"` // specific config
}

// helper to decode map[string]any to a target type using mapstructure
// stringToDurationHook converts string values like "150ms", "2s" to time.Duration
func stringToDurationHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		// Only handle string -> time.Duration
		if from.Kind() == reflect.String && to == reflect.TypeOf(time.Duration(0)) {
			s := strings.TrimSpace(data.(string))
			if s == "" {
				return time.Duration(0), nil
			}
			d, err := time.ParseDuration(s)
			if err != nil {
				return nil, fmt.Errorf("cannot parse value as 'time.Duration': %w", err)
			}
			return d, nil
		}
		return data, nil
	}
}

func decodeTo[T any](m map[string]any) (T, error) {
	var out T
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(stringToDurationHook()),
		Result:           &out,
	})
	if err != nil {
		return out, err
	}
	if err := dec.Decode(m); err != nil {
		return out, err
	}
	// If the target type implements a Validate() error method, call it.
	if v, ok := any(&out).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			var zero T
			return zero, err
		}
	}
	return out, nil
}

// decodeProcessEntry decodes and validates a ProcessConfig entry (process or cronjob).
// ctx is used to improve error messages with the source (e.g., filename or "inline processes").
func decodeProcessEntry(pc ProcessConfig, ctx string) (core.Spec, *core.CronJob, error) {
	var zero core.Spec
	typ := strings.ToLower(strings.TrimSpace(pc.Type))
	switch typ {
	case "", "process":
		sp, err := decodeTo[core.Spec](pc.Spec)
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
		jb, err := decodeTo[core.CronJob](pc.Spec)
		if err != nil {
			return zero, nil, fmt.Errorf("decode cronjob spec in %s: %w", ctx, err)
		}
		if strings.TrimSpace(jb.Name) == "" {
			jb.Name = strings.TrimSpace(jb.JobTemplate.Name)
		}
		if strings.TrimSpace(jb.Name) == "" {
			return zero, nil, fmt.Errorf("%s: cronjob requires name", ctx)
		}
		if strings.TrimSpace(jb.JobTemplate.Command) == "" && len(jb.JobTemplate.Args) == 0 {
			return zero, nil, fmt.Errorf("%s: cronjob %q requires command or args", ctx, jb.Name)
		}
		if strings.TrimSpace(jb.Schedule) == "" {
			return zero, nil, fmt.Errorf("%s: cronjob %q requires schedule", ctx, jb.Name)
		}
		processSpec := jb.JobTemplate.ToProcessSpec()
		return *processSpec, &jb, nil
	default:
		return zero, nil, fmt.Errorf("%s: unknown process type %q (allowed: process, cronjob)", ctx, pc.Type)
	}
}

func LoadConfig(configPath string) (*LoadedConfig, error) {
	var raw Config

	if err := parseConfigFile(configPath, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := validateConfig(&raw); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	config := &LoadedConfig{Config: raw, configPath: configPath}
	resolveConfigPaths(&config.Config, filepath.Dir(configPath))

	// Initialize aggregated fields
	config.Specs = make([]core.Spec, 0)
	config.CronJobs = []core.CronJob{}

	// 1) Inline processes: discriminated union decoding (refactored)
	for _, pc := range config.Processes {
		spec, job, err := decodeProcessEntry(pc, "inline processes")
		if err != nil {
			return nil, err
		}
		if job != nil {
			resolveCronJobPaths(job, filepath.Dir(configPath))
			spec = *job.JobTemplate.ToProcessSpec()
		} else {
			resolveSpecPaths(&spec, filepath.Dir(configPath))
		}
		// convert detectors after decode
		if err := convertDetectorConfigs(&spec); err != nil {
			return nil, fmt.Errorf("failed to convert detectors for process %s: %w", spec.Name, err)
		}
		config.Specs = append(config.Specs, spec)
		if job != nil {
			if err := convertDetectorConfigs(job.JobTemplate.ToProcessSpec()); err != nil {
				return nil, fmt.Errorf("failed to convert detectors for cronjob %s: %w", job.Name, err)
			}
			config.CronJobs = append(config.CronJobs, *job)
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

	config.ResolvedProgramsDirectory = programsDir

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
			jobSpec := j.JobTemplate.ToProcessSpec()
			if err := convertDetectorConfigs(jobSpec); err != nil {
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

	// Apply default pid_dir to specs if configured and PIDFile is empty
	if strings.TrimSpace(config.PIDDir) != "" {
		pidDir := config.PIDDir
		if !filepath.IsAbs(pidDir) {
			pidDir = filepath.Join(filepath.Dir(configPath), pidDir)
		}
		for i := range config.Specs {
			if strings.TrimSpace(config.Specs[i].PIDFile) == "" {
				config.Specs[i].PIDFile = filepath.Join(pidDir, config.Specs[i].Name+".pid")
			}
		}
	}

	// Build groups using the aggregated specs
	if err := validateUniqueRuntimeEntries(config.Specs, config.CronJobs); err != nil {
		return nil, err
	}
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

func validateUniqueRuntimeEntries(specs []core.Spec, jobs []core.CronJob) error {
	processNames := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if _, exists := processNames[spec.Name]; exists {
			return fmt.Errorf("duplicate process name %q", spec.Name)
		}
		processNames[spec.Name] = struct{}{}
	}
	jobNames := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if _, exists := jobNames[job.Name]; exists {
			return fmt.Errorf("duplicate cronjob name %q", job.Name)
		}
		jobNames[job.Name] = struct{}{}
	}
	return nil
}

func resolveConfigPaths(cfg *Config, baseDir string) {
	resolve := func(path string) string {
		if path == "" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Clean(filepath.Join(baseDir, path))
	}

	for i := range cfg.EnvFiles {
		cfg.EnvFiles[i] = resolve(cfg.EnvFiles[i])
	}
	cfg.PIDDir = resolve(cfg.PIDDir)
	if cfg.Daemon != nil {
		cfg.Daemon.PIDFile = resolve(cfg.Daemon.PIDFile)
		cfg.Daemon.LogFile = resolve(cfg.Daemon.LogFile)
	}
	if cfg.Server != nil {
		if cfg.Server.TLS != nil {
			cfg.Server.TLS.CertFile = resolve(cfg.Server.TLS.CertFile)
			cfg.Server.TLS.KeyFile = resolve(cfg.Server.TLS.KeyFile)
			cfg.Server.TLS.Dir = resolve(cfg.Server.TLS.Dir)
		}
		if cfg.Server.Auth != nil && strings.EqualFold(cfg.Server.Auth.Store.Type, "sqlite") {
			cfg.Server.Auth.Store.Path = resolve(cfg.Server.Auth.Store.Path)
		}
	}
	if cfg.History != nil && cfg.History.Stores.SQLite != nil {
		dsn := cfg.History.Stores.SQLite.DSN
		if dsn != "" && dsn != ":memory:" && !strings.Contains(dsn, "://") {
			cfg.History.Stores.SQLite.DSN = resolve(dsn)
		}
	}
}

func parseConfigFile(configPath string, out interface{}) error {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.UnmarshalExact(out); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// loadProgramEntries loads program entries from the programs directory using the same
// discriminated-union format as inline [[processes]] blocks: {type, spec}.
// Supported file extensions: toml, yaml/yml, json. No legacy plain core.Spec files supported.
func loadProgramEntries(programsDir string) ([]core.Spec, []core.CronJob, error) {
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

	var specs []core.Spec
	var jobs []core.CronJob
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
		if err := v.UnmarshalExact(&pc); err != nil {
			return nil, nil, fmt.Errorf("unmarshal %s: %w", full, err)
		}

		sp, jb, err := decodeProcessEntry(pc, full)
		if err != nil {
			return nil, nil, err
		}
		if jb != nil {
			resolveCronJobPaths(jb, filepath.Dir(full))
			sp = *jb.JobTemplate.ToProcessSpec()
			jobs = append(jobs, *jb)
		} else {
			resolveSpecPaths(&sp, filepath.Dir(full))
		}
		specs = append(specs, sp)
	}
	return specs, jobs, nil
}

func resolveSpecPaths(spec *core.Spec, baseDir string) {
	resolve := func(path string) string {
		if path == "" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Clean(filepath.Join(baseDir, path))
	}
	spec.WorkDir = resolve(spec.WorkDir)
	spec.PIDFile = resolve(spec.PIDFile)
	spec.Log.File.Dir = resolve(spec.Log.File.Dir)
	spec.Log.File.StdoutPath = resolve(spec.Log.File.StdoutPath)
	spec.Log.File.StderrPath = resolve(spec.Log.File.StderrPath)
	for i := range spec.DetectorConfigs {
		if spec.DetectorConfigs[i].Type == "pidfile" {
			spec.DetectorConfigs[i].Path = resolve(spec.DetectorConfigs[i].Path)
		}
	}
	resolveLifecyclePaths(&spec.Lifecycle, resolve)
}

func resolveCronJobPaths(job *core.CronJob, baseDir string) {
	processSpec := job.JobTemplate.ToProcessSpec()
	resolveSpecPaths(processSpec, baseDir)
	job.JobTemplate.WorkDir = processSpec.WorkDir
	job.JobTemplate.Log = processSpec.Log
	job.JobTemplate.Lifecycle = processSpec.Lifecycle
	resolve := func(path string) string {
		if path == "" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Clean(filepath.Join(baseDir, path))
	}
	resolveLifecyclePaths(&job.Lifecycle, resolve)
}

func resolveLifecyclePaths(hooks *core.LifecycleHooks, resolve func(string) string) {
	groups := [][]core.Hook{hooks.PreStart, hooks.PostStart, hooks.PreStop, hooks.PostStop}
	for _, group := range groups {
		for i := range group {
			group[i].WorkDir = resolve(group[i].WorkDir)
		}
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Server != nil {
		if cfg.Server.TLS != nil {
			validTLSVersion := func(value string) bool {
				switch value {
				case "", "default", "1.2", "1.3", "TLS1.2", "TLS1.3", "tls1.2", "tls1.3":
					return true
				default:
					return false
				}
			}
			if !validTLSVersion(cfg.Server.TLS.MinVersion) || !validTLSVersion(cfg.Server.TLS.MaxVersion) {
				return fmt.Errorf("server.tls min_version and max_version must be 1.2 or 1.3")
			}
			if cfg.Server.TLS.MinVersion == "1.3" && cfg.Server.TLS.MaxVersion == "1.2" {
				return fmt.Errorf("server.tls.max_version must not be lower than min_version")
			}
		}
		if auth := cfg.Server.Auth; auth != nil && auth.Enabled {
			switch strings.ToLower(auth.Store.Type) {
			case "sqlite":
				if strings.TrimSpace(auth.Store.Path) == "" {
					return fmt.Errorf("server.auth.store.path is required for sqlite")
				}
			case "postgres", "postgresql":
				if strings.TrimSpace(auth.Store.Database) == "" {
					return fmt.Errorf("server.auth.store.database is required for postgresql")
				}
			default:
				return fmt.Errorf("server.auth.store.type must be sqlite or postgresql")
			}
			if auth.Store.MaxOpenConns > 0 && auth.Store.MaxIdleConns > auth.Store.MaxOpenConns {
				return fmt.Errorf("server.auth.store.max_idle_conns must not exceed max_open_conns")
			}
		}
	}
	if cfg.Metrics != nil && cfg.Metrics.ProcessMetrics != nil {
		process := cfg.Metrics.ProcessMetrics
		if process.Interval < 0 || process.MaxHistory < 0 {
			return fmt.Errorf("metrics.process_metrics interval and max_history must not be negative")
		}
	}

	if cfg.History == nil || !cfg.History.Enabled {
		return nil
	}

	enabled := map[string]bool{}
	if store := cfg.History.Stores.SQLite; store != nil && store.Enabled {
		enabled["sqlite"] = true
		if store.Retention < 0 || store.CleanupInterval < 0 {
			return fmt.Errorf("history.stores.sqlite retention durations must not be negative")
		}
	}
	if store := cfg.History.Stores.Postgres; store != nil && store.Enabled {
		enabled["postgres"] = true
		if strings.TrimSpace(store.DSN) == "" {
			return fmt.Errorf("history.stores.postgres.dsn is required")
		}
		if store.Retention < 0 || store.CleanupInterval < 0 {
			return fmt.Errorf("history.stores.postgres retention durations must not be negative")
		}
	}
	if store := cfg.History.Stores.ClickHouse; store != nil && store.Enabled {
		enabled["clickhouse"] = true
		if strings.TrimSpace(store.DSN) == "" {
			return fmt.Errorf("history.stores.clickhouse.dsn is required")
		}
		if store.Retention < 0 || store.CleanupInterval < 0 {
			return fmt.Errorf("history.stores.clickhouse retention durations must not be negative")
		}
	}
	if store := cfg.History.Stores.OpenSearch; store != nil && store.Enabled {
		enabled["opensearch"] = true
		if strings.TrimSpace(store.URL) == "" || strings.TrimSpace(store.Index) == "" {
			return fmt.Errorf("history.stores.opensearch.url and index are required")
		}
		if store.Retention < 0 || store.CleanupInterval < 0 {
			return fmt.Errorf("history.stores.opensearch retention durations must not be negative")
		}
	}
	if len(enabled) == 0 {
		return fmt.Errorf("history.enabled requires at least one enabled store")
	}
	if strings.TrimSpace(cfg.History.Primary) == "" {
		return fmt.Errorf("history.primary is required")
	}
	if !enabled[cfg.History.Primary] {
		return fmt.Errorf("history.primary %q must name an enabled store", cfg.History.Primary)
	}
	return nil
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

func applyGlobalLogDefaults(cfg *LoadedConfig) error {
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

	globalDir := makeAbs(cfg.Log.File.Dir)
	globalStdout := makeAbs(cfg.Log.File.StdoutPath)
	globalStderr := makeAbs(cfg.Log.File.StderrPath)

	apply := func(sp *core.Spec) {
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
		if sp.Log.File.MaxSizeMB == 0 && cfg.Log.File.MaxSizeMB > 0 {
			sp.Log.File.MaxSizeMB = cfg.Log.File.MaxSizeMB
		}
		if sp.Log.File.MaxBackups == 0 && cfg.Log.File.MaxBackups > 0 {
			sp.Log.File.MaxBackups = cfg.Log.File.MaxBackups
		}
		if sp.Log.File.MaxAgeDays == 0 && cfg.Log.File.MaxAgeDays > 0 {
			sp.Log.File.MaxAgeDays = cfg.Log.File.MaxAgeDays
		}
		// Compress default copies boolean as-is only when any path configured
		if noPathsSet {
			// If we just set paths above, respect global Compress
			sp.Log.File.Compress = cfg.Log.File.Compress
		}
	}

	for i := range cfg.Specs {
		apply(&cfg.Specs[i])
	}
	for i := range cfg.CronJobs {
		jobSpec := cfg.CronJobs[i].JobTemplate.ToProcessSpec()
		apply(jobSpec)
		// Update the JobTemplate with the modified process spec
		// Note: We need to manually copy back the modified fields
		cfg.CronJobs[i].JobTemplate.Name = jobSpec.Name
		cfg.CronJobs[i].JobTemplate.Command = jobSpec.Command
		cfg.CronJobs[i].JobTemplate.WorkDir = jobSpec.WorkDir
		cfg.CronJobs[i].JobTemplate.Env = jobSpec.Env
		cfg.CronJobs[i].JobTemplate.Log = jobSpec.Log
	}
	return nil
}

func buildGroups(groupConfigs []GroupConfig, specs []core.Spec) ([]core.ServiceGroup, error) {
	specMap := make(map[string]core.Spec, len(specs))
	for _, spec := range specs {
		specMap[spec.Name] = spec
	}

	groups := make([]core.ServiceGroup, 0, len(groupConfigs))
	groupNames := make(map[string]struct{}, len(groupConfigs))
	for _, gc := range groupConfigs {
		if gc.Name == "" {
			return nil, fmt.Errorf("group requires name")
		}
		if len(gc.Members) == 0 {
			return nil, fmt.Errorf("group %s requires members", gc.Name)
		}
		if _, exists := groupNames[gc.Name]; exists {
			return nil, fmt.Errorf("duplicate group name %q", gc.Name)
		}
		groupNames[gc.Name] = struct{}{}

		memberSpecs := make([]core.Spec, 0, len(gc.Members))
		for _, memberName := range gc.Members {
			spec, exists := specMap[memberName]
			if !exists {
				return nil, fmt.Errorf("group %s references unknown member %s", gc.Name, memberName)
			}
			memberSpecs = append(memberSpecs, spec)
		}

		groups = append(groups, core.ServiceGroup{
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
func convertDetectorConfigs(spec *core.Spec) error {
	if len(spec.DetectorConfigs) == 0 {
		return nil
	}

	spec.Detectors = make([]core.Detector, len(spec.DetectorConfigs))
	for i, config := range spec.DetectorConfigs {
		switch config.Type {
		case "pidfile":
			if config.Path == "" {
				return fmt.Errorf("pidfile detector requires 'path' field")
			}
			spec.Detectors[i] = &core.PIDFileDetector{PIDFile: config.Path}
		case "command":
			if config.Command == "" {
				return fmt.Errorf("command detector requires 'command' field")
			}
			spec.Detectors[i] = &core.CommandDetector{Command: config.Command}
		default:
			return fmt.Errorf("unknown detector type: %s", config.Type)
		}
	}

	return nil
}
