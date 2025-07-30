package config

import (
    "fmt"
    "github.com/spf13/viper"
)

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