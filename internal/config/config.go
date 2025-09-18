package config

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// AppConfig holds all application-wide configuration from environment variables.
type AppConfig struct {
	DockerUser     string `mapstructure:"docker_user"`
	DockerPass     string `mapstructure:"docker_pass"`
	GithubToken    string `mapstructure:"github_token"`
	TelegramToken  string `mapstructure:"telegram_token"`
	TelegramChatID string `mapstructure:"telegram_chat_id"`
	Git            struct {
		RepoURL       string `mapstructure:"repo_url"`
		Branch        string `mapstructure:"branch"`
		SSHKeyPath    string `mapstructure:"ssh_key_path"`
		LocalRepoPath string `mapstructure:"local_repo_path"`
		AuthorName    string `mapstructure:"author_name"`
		AuthorEmail   string `mapstructure:"author_email"`
	} `mapstructure:"git"`

	Checker struct {
		WatchFolder       string   `mapstructure:"watch_folder"`
		ProposedTagsCount int      `mapstructure:"proposed_tags_count"`
		IntervalMinutes   int      `mapstructure:"interval_minutes"`
		SkipListFile      string   `mapstructure:"skip_list_file"`
		IgnoreKeywords    []string `mapstructure:"ignore_keywords"`
	} `mapstructure:"checker"`
}

// LoadConfig loads configuration from a YAML file and overrides values with environment variables.
func LoadConfig(path string) (*AppConfig, error) {
	// Bind environment variables to your struct fields
	// This allows env vars to override values from the config file
	viper.BindEnv("docker_user", "CONTROLLA__DOCKERHUB__USERNAME")
	viper.BindEnv("docker_pass", "CONTROLLA__DOCKERHUB__PASSWORD")
	viper.BindEnv("github_token", "CONTROLLA__GITHUB__TOKEN")
	viper.BindEnv("telegram_token", "CONTROLLA__TELEGRAM__TOKEN")
	viper.BindEnv("telegram_chat_id", "CONTROLLA__TELEGRAM__CHAT__ID")
	viper.BindEnv("git.ssh_key_path", "SSH_KEY_PATH")

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, but we can continue by loading from env vars only
			logrus.Warn("Config file not found. Loading configuration from environment variables.")
		} else {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Unmarshal the config into the struct
	var config AppConfig
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Simple validation
	if config.DockerUser == "" && config.GithubToken == "" {
		logrus.Warning("Warning: No Docker Hub or GitHub credentials found. Some features may not work.")
	}

	return &config, nil
}
