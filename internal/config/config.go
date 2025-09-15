package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type RegistryAuth struct {
	Token    string `yaml:"token"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RegistryConfig struct {
	GHCR      RegistryAuth `yaml:"ghcr"`
	DockerHub RegistryAuth `yaml:"dockerhub"`
	AWS       RegistryAuth `yaml:"aws"`
}

type Config struct {
	Registry RegistryConfig `yaml:"registry"`

	Telegram struct {
		Token  string `mapstructure:"token"`
		ChatID int64  `mapstructure:"chat_id"`
	} `mapstructure:"telegram"`

	Git struct {
		RepoURL    string `mapstructure:"repo_url"`
		Branch     string `mapstructure:"branch"`
		SSHKeyPath string `mapstructure:"ssh_key_path"`
	} `mapstructure:"git"`

	Checker struct {
		IntervalMinutes int    `mapstructure:"interval_minutes"`
		SkipListFile    string `mapstructure:"skip_list_file"`
	} `mapstructure:"checker"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
