package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/viper"

	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/process_group"
)

type Config struct {
	UseOSEnv bool           `mapstructure:"use_os_env"`
	EnvFiles []string       `mapstructure:"env_files"`
	Env      []string       `mapstructure:"env"`
	Groups   []GroupConfig  `mapstructure:"groups"`
	Store    *StoreConfig   `mapstructure:"store"`
	History  *HistoryConfig `mapstructure:"history"`
	Metrics  *MetricsConfig `mapstructure:"metrics"`
	Log      *LogConfig     `mapstructure:"log"`
	Server   *ServerConfig  `mapstructure:"server"`

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

	config.Specs = []process.Spec{}
	config.CronJobs = []CronJob{}

	globalEnv, err := computeGlobalEnv(config.UseOSEnv, config.EnvFiles, config.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to compute global env: %w", err)
	}
	config.GlobalEnv = globalEnv

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
