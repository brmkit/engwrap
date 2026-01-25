package engwrap

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// config represents an environment configuration
type Config struct {
	Name          string   `yaml:"name"`
	Image         string   `yaml:"image"`
	InitCommands  []string `yaml:"init_commands"`
	BashrcCustoms []string `yaml:"bashrc_customs"`

	// Networking & Advanced Config
	NetworkMode string   `yaml:"network_mode,omitempty"`
	Mounts      []string `yaml:"mounts,omitempty"`
}

// GetWorkspacePath returns the workspace path for this config
// Always returns $HOME/.engwrap/work/<name>
func (c *Config) GetWorkspacePath() (string, error) {
	workDir, err := GetEngwrapWorkDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(workDir, c.Name), nil
}

// load config from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	if config.Name == "" {
		return nil, fmt.Errorf("missing 'name' field in config file")
	}
	if config.Image == "" {
		return nil, fmt.Errorf("missing 'image' field in config file")
	}

	return &config, nil
}
