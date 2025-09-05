package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/process_group"
	"github.com/spf13/viper"
)

type CronJob struct {
	Name      string
	Spec      process.Spec
	Schedule  string
	Singleton bool
}

// FileConfig represents the top-level TOML structure.
// See internal docs for example in previous location.

type FileConfig struct {
	Env       []string      `toml:"env" mapstructure:"env"`
	EnvFiles  []string      `toml:"env_files" mapstructure:"env_files"`
	UseOSEnv  bool          `toml:"use_os_env" mapstructure:"use_os_env"`
	Log       *LogConfig    `toml:"log" mapstructure:"log"`
	Processes []ProcConfig  `toml:"processes" mapstructure:"processes"`
	Groups    []GroupConfig `toml:"groups" mapstructure:"groups"`
}

type LogConfig struct {
	Dir        string `toml:"dir" mapstructure:"dir"`
	Stdout     string `toml:"stdout" mapstructure:"stdout"`
	Stderr     string `toml:"stderr" mapstructure:"stderr"`
	MaxSizeMB  int    `toml:"max_size_mb" mapstructure:"max_size_mb"`
	MaxBackups int    `toml:"max_backups" mapstructure:"max_backups"`
	MaxAgeDays int    `toml:"max_age_days" mapstructure:"max_age_days"`
	Compress   bool   `toml:"compress" mapstructure:"compress"`
}

type ProcConfig struct {
	Name            string          `toml:"name" mapstructure:"name"`
	Command         string          `toml:"command" mapstructure:"command"`
	WorkDir         string          `toml:"workdir" mapstructure:"workdir"`
	Env             []string        `toml:"env" mapstructure:"env"`
	PIDFile         string          `toml:"pidfile" mapstructure:"pidfile"`
	RetryCount      int             `toml:"retries" mapstructure:"retries"`
	RetryInterval   time.Duration   `toml:"retry_interval" mapstructure:"retry_interval"`
	StartDuration   time.Duration   `toml:"startsecs" mapstructure:"startsecs"`
	AutoRestart     bool            `toml:"autorestart" mapstructure:"autorestart"`
	RestartInterval time.Duration   `toml:"restart_interval" mapstructure:"restart_interval"`
	Instances       int             `toml:"instances" mapstructure:"instances"`
	Detectors       []DetectorEntry `toml:"detectors" mapstructure:"detectors"`
	Schedule        string          `toml:"schedule" mapstructure:"schedule"`
	Singleton       *bool           `toml:"singleton" mapstructure:"singleton"`
	Log             *LogConfig      `toml:"log" mapstructure:"log"`
}

type DetectorEntry struct {
	Type    string `toml:"type" mapstructure:"type"`
	Path    string `toml:"path" mapstructure:"path"`
	PID     int    `toml:"pid" mapstructure:"pid"`
	Command string `toml:"command" mapstructure:"command"`
}

type GroupConfig struct {
	Name    string   `toml:"name" mapstructure:"name"`
	Members []string `toml:"members" mapstructure:"members"`
}

// LoadEnvFromTOML parses only the top-level env list from TOML.
func LoadEnvFromTOML(path string) ([]string, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := v.Unmarshal(&fc); err != nil {
		return nil, err
	}
	return fc.Env, nil
}

// LoadGlobalEnv merges env from config: top-level env, env_files contents, and optionally OS env when UseOSEnv is true.
// Precedence: OS env (when enabled) provides base; then apply file vars; then top-level env list overrides last.
func LoadGlobalEnv(path string) ([]string, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := v.Unmarshal(&fc); err != nil {
		return nil, err
	}
	m := make(map[string]string)
	// base: optional OS env
	if fc.UseOSEnv {
		for _, kv := range os.Environ() {
			if i := strings.IndexByte(kv, '='); i >= 0 {
				k := kv[:i]
				v := kv[i+1:]
				m[k] = v
			}
		}
	}
	// load files in order
	for _, p := range fc.EnvFiles {
		pairs, err := loadEnvFile(p)
		if err != nil {
			return nil, err
		}
		for k, v := range pairs {
			m[k] = v
		}
	}
	// apply top-level env overrides
	for _, kv := range fc.Env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			m[k] = v
		}
	}
	// build slice
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out, nil
}

// LoadEnvFile parses a simple .env file and returns a slice of "KEY=VALUE" entries.
func LoadEnvFile(path string) ([]string, error) {
	m, err := loadEnvFile(path)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out, nil
}

// loadEnvFile parses a simple .env file with KEY=VALUE lines (no export, no quotes). Lines starting with # are ignored.
func loadEnvFile(path string) (map[string]string, error) {
	// Mitigate G304: sanitize user-provided path by cleaning it before use.
	clean := filepath.Clean(path)
	b, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '='); i >= 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.TrimSpace(line[i+1:])
			m[k] = v
		}
	}
	return m, nil
}

// LoadSpecsFromTOML parses a TOML config file into a slice of process.Spec.
func LoadSpecsFromTOML(path string) ([]process.Spec, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := v.Unmarshal(&fc); err != nil {
		return nil, err
	}
	result := make([]process.Spec, 0, len(fc.Processes))
	for _, pc := range fc.Processes {
		// detectors
		dets := make([]detector.Detector, 0, len(pc.Detectors))
		for _, d := range pc.Detectors {
			switch d.Type {
			case "pidfile":
				if d.Path == "" {
					return nil, fmt.Errorf("detector pidfile requires path for process %s", pc.Name)
				}
				dets = append(dets, detector.PIDFileDetector{PIDFile: d.Path})
			case "pid":
				if d.PID <= 0 {
					return nil, fmt.Errorf("detector pid requires positive pid for process %s", pc.Name)
				}
				dets = append(dets, detector.PIDDetector{PID: d.PID})
			case "command":
				if d.Command == "" {
					return nil, fmt.Errorf("detector command requires command for process %s", pc.Name)
				}
				dets = append(dets, detector.CommandDetector{Command: d.Command})
			default:
				return nil, fmt.Errorf("unknown detector type %q for process %s", d.Type, pc.Name)
			}
		}
		// logging config: start with top-level defaults then override with per-process
		var logCfg logger.Config
		if fc.Log != nil {
			logCfg = logger.Config{
				Dir:        fc.Log.Dir,
				StdoutPath: fc.Log.Stdout,
				StderrPath: fc.Log.Stderr,
				MaxSizeMB:  fc.Log.MaxSizeMB,
				MaxBackups: fc.Log.MaxBackups,
				MaxAgeDays: fc.Log.MaxAgeDays,
				Compress:   fc.Log.Compress,
			}
		}
		if pc.Log != nil {
			if pc.Log.Dir != "" {
				logCfg.Dir = pc.Log.Dir
			}
			if pc.Log.Stdout != "" {
				logCfg.StdoutPath = pc.Log.Stdout
			}
			if pc.Log.Stderr != "" {
				logCfg.StderrPath = pc.Log.Stderr
			}
			if pc.Log.MaxSizeMB != 0 {
				logCfg.MaxSizeMB = pc.Log.MaxSizeMB
			}
			if pc.Log.MaxBackups != 0 {
				logCfg.MaxBackups = pc.Log.MaxBackups
			}
			if pc.Log.MaxAgeDays != 0 {
				logCfg.MaxAgeDays = pc.Log.MaxAgeDays
			}
			if pc.Log.Compress {
				logCfg.Compress = true
			}
		}

		s := process.Spec{
			Name:            pc.Name,
			Command:         pc.Command,
			WorkDir:         pc.WorkDir,
			Env:             pc.Env,
			PIDFile:         pc.PIDFile,
			RetryCount:      pc.RetryCount,
			RetryInterval:   pc.RetryInterval,
			StartDuration:   pc.StartDuration,
			AutoRestart:     pc.AutoRestart,
			RestartInterval: pc.RestartInterval,
			Instances:       pc.Instances,
			Detectors:       dets,
			Log:             logCfg,
		}
		result = append(result, s)
	}
	return result, nil
}

// LoadGroupsFromTOML parses group definitions and returns process_group.GroupSpec list.
func LoadGroupsFromTOML(path string) ([]process_group.GroupSpec, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := v.Unmarshal(&fc); err != nil {
		return nil, err
	}
	// Build map name->Spec for member lookup
	specs, err := LoadSpecsFromTOML(path)
	if err != nil {
		return nil, err
	}
	m := make(map[string]process.Spec, len(specs))
	for _, s := range specs {
		m[s.Name] = s
	}
	groups := make([]process_group.GroupSpec, 0, len(fc.Groups))
	for _, gc := range fc.Groups {
		if gc.Name == "" {
			return nil, fmt.Errorf("group requires name")
		}
		if len(gc.Members) == 0 {
			return nil, fmt.Errorf("group %s must list members", gc.Name)
		}
		members := make([]process.Spec, 0, len(gc.Members))
		for _, mn := range gc.Members {
			s, ok := m[mn]
			if !ok {
				return nil, fmt.Errorf("group %s references unknown process %s", gc.Name, mn)
			}
			members = append(members, s)
		}
		groups = append(groups, process_group.GroupSpec{Name: gc.Name, Members: members})
	}
	return groups, nil
}

// LoadCronJobsFromTOML reads the same TOML but returns only entries that define a schedule.
// It validates cron-specific constraints (autorestart must be false; instances must be 1).
func LoadCronJobsFromTOML(path string) ([]CronJob, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := v.Unmarshal(&fc); err != nil {
		return nil, err
	}
	jobs := make([]CronJob, 0)
	for _, pc := range fc.Processes {
		if pc.Schedule == "" {
			continue
		}
		if pc.AutoRestart {
			return nil, fmt.Errorf("cron job %s cannot set autorestart=true", pc.Name)
		}
		if pc.Instances > 1 {
			return nil, fmt.Errorf("cron job %s cannot set instances > 1", pc.Name)
		}
		// detectors
		dets := make([]detector.Detector, 0, len(pc.Detectors))
		for _, d := range pc.Detectors {
			switch d.Type {
			case "pidfile":
				if d.Path == "" {
					return nil, fmt.Errorf("detector pidfile requires path for process %s", pc.Name)
				}
				dets = append(dets, detector.PIDFileDetector{PIDFile: d.Path})
			case "pid":
				if d.PID <= 0 {
					return nil, fmt.Errorf("detector pid requires positive pid for process %s", pc.Name)
				}
				dets = append(dets, detector.PIDDetector{PID: d.PID})
			case "command":
				if d.Command == "" {
					return nil, fmt.Errorf("detector command requires command for process %s", pc.Name)
				}
				dets = append(dets, detector.CommandDetector{Command: d.Command})
			default:
				return nil, fmt.Errorf("unknown detector type %q for process %s", d.Type, pc.Name)
			}
		}
		s := process.Spec{
			Name:            pc.Name,
			Command:         pc.Command,
			WorkDir:         pc.WorkDir,
			Env:             pc.Env,
			PIDFile:         pc.PIDFile,
			RetryCount:      pc.RetryCount,
			RetryInterval:   pc.RetryInterval,
			StartDuration:   pc.StartDuration,
			AutoRestart:     false, // enforce
			RestartInterval: 0,
			Instances:       1,
			Detectors:       dets,
			Log:             logger.Config{},
		}
		singleton := true
		if pc.Singleton != nil {
			singleton = *pc.Singleton
		}
		jobs = append(jobs, CronJob{Name: pc.Name, Spec: s, Schedule: pc.Schedule, Singleton: singleton})
	}
	return jobs, nil
}
