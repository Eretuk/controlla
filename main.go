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
	logrus.Info("Config file loaded")

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
			parsed := checker.ParseImageName(img.Name)

			digest, err := checker.GetImageDigest(parsed, cfg)
			if err != nil {
				logrus.Warnf("Failed to get digest for %s: %v", parsed.Full, err)
				continue
			}

			logrus.Infof("[%s/%s] digest for %s: %s", img.Host, img.Service, parsed.Full, digest)
		}

		time.Sleep(time.Duration(cfg.Checker.IntervalMinutes) * time.Minute)
	}

}
