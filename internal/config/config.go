package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// AppConfig holds all application-wide configuration from environment variables.
type AppConfig struct {
	DockerUser     string
	DockerPass     string
	GithubToken    string
	TelegramToken  string
	TelegramChatID string
}

type Config struct {
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
		WatchFolder       string `mapstructure:"watch_folder"`
		ProposedTagsCount int    `mapstructure:"proposed_tags_count"`
		IntervalMinutes   int    `mapstructure:"interval_minutes"`
		SkipListFile      string `mapstructure:"skip_list_file"`
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

// LoadFromEnv reads configuration values from environment variables.
func LoadFromEnv() AppConfig {
	cfg := AppConfig{
		DockerUser:     os.Getenv("CONTROLLA__DOCKERHUB__USERNAME"),
		DockerPass:     os.Getenv("CONTROLLA__DOCKERHUB__PASSWORD"),
		GithubToken:    os.Getenv("CONTROLLA__GITHUB__TOKEN"),
		TelegramToken:  os.Getenv("CONTROLLA__TELEGRAM__TOKEN"),
		TelegramChatID: os.Getenv("CONTROLLA__TELEGRAM__CHAT__ID"),
	}

	// Simple validation to ensure we have at least one token
	if cfg.DockerUser == "" && cfg.GithubToken == "" {
		logrus.Warning("Warning: No Docker Hub or GitHub credentials found. Some features may not work.")
	}

	return cfg
}
