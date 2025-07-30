package main

import (
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	logrus.Info("Loading config.yaml...")
	cfg, err := config.Load("config.yaml")
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}
	logrus.Info("Done")

	logrus.Infof("Controlla started. Checking every %d minutes.", cfg.Checker.IntervalMinutes)

	for {
		logrus.Info("Checking for updates...")
		repoPath := "./repos/myrepo"
		if err := gitrepo.CloneOrPull(cfg.Git.RepoURL, cfg.Git.Branch, repoPath, cfg.Git.SSHKeyPath); err != nil {
			logrus.Fatalf("Failed to sync repo: %v", err)
		}
		logrus.Info("Repo synced")

		repoServices := "./repos/myrepo/services"
		images, err := checker.FindImagesInCompose(repoServices)
		if err != nil {
			logrus.Fatalf("failed to scan compose files: %v", err)
		}

		logrus.Infof("Found %d images:", len(images))
		for _, img := range images {
			logrus.Infof("  [%s/%s] %s (%s)", img.Host, img.Service, img.Name, img.Path)
		}
		time.Sleep(time.Duration(cfg.Checker.IntervalMinutes) * time.Minute)
	}

}
