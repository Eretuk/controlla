package main

import (
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"controlla/internal/registry"
	"controlla/internal/versionutils"
	"log"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	logrus.Info("Loading config.yaml.")
	cfg, err := config.Load("config.yaml")
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}
	logrus.Info("Config file has loaded.")

	// Load configuration from environment variables
	logrus.Info("Start loading env variables")
	appConfig := config.LoadFromEnv()
	logrus.Info("Env variables loaded")
	repoPath := "./repos/myrepo"

	log.Printf("Cloning repository %s to %s", cfg.Git.RepoURL, repoPath)
	err = gitrepo.CloneOrPull(cfg.Git.RepoURL, cfg.Git.Branch, repoPath, cfg.Git.SSHKeyPath)
	if err != nil {
		log.Fatalf("Failed to clone repository: %v", err)
	}
	log.Println("Repository cloned successfully.")

	// Now, define the folder to watch based on your config.
	// We construct the full path by joining the local repo path and the watch folder from config.
	watchPath := filepath.Join(repoPath, cfg.Checker.WatchFolder)

	// Find all relevant files within the specified watch folder.
	log.Printf("Scanning for files in %s...", watchPath)
	foundFiles, err := checker.FindRelevantFiles(watchPath)
	if err != nil {
		log.Fatalf("Failed to find files in watch folder: %v", err)
	}

	log.Printf("Found %d relevant files:", len(foundFiles))
	for _, file := range foundFiles {
		log.Println(file)
	}
	// log.Println("Scanning cloned repository for images...")
	// foundImages, err := checker.FindImagesInCompose(repoPath)
	// if err != nil {
	// 	log.Fatalf("Failed to find images in repository: %v", err)
	// }

	// log.Printf("Found %d images: %v", len(foundImages), foundImages)
	// logrus.Infof("Controlla started. Checking every %d minutes.", cfg.Checker.IntervalMinutes)

	// 2. Create clients
	dockerClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
	// ghcrClient := registry.NewGhcrClient(appConfig.GithubToken)

	// 3. Use the clients to get tags (example)
	if appConfig.DockerUser != "" {
		dockerImage := "authelia/authelia"
		tags, err := dockerClient.GetTags(dockerImage)
		if err != nil {
			log.Printf("Error getting Docker Hub tags for %s: %v", dockerImage, err)
			return
		}
		latestTags := versionutils.SortAndFilterTags(tags, 5)
		// Getiing results
		log.Printf("Docker Hub tags for %s: %v", dockerImage, latestTags)

	}

	// if appConfig.GithubToken != "" {
	// 	ghcrImage := "authelia/authelia" // your ghcr image
	// 	tags, err := ghcrClient.GetTags(ghcrImage)
	// 	if err != nil {
	// 		log.Printf("Error getting GHCR tags for %s: %v", ghcrImage, err)
	// 	} else {
	// 		log.Printf("GHCR tags for %s: %v", ghcrImage, tags)
	// 	}
	// }
	// for {
	// 	logrus.Info("Checking for updates...")
	// 	repoPath := "./repos/myrepo"
	// 	if err := gitrepo.CloneOrPull(cfg.Git.RepoURL, cfg.Git.Branch, repoPath, cfg.Git.SSHKeyPath); err != nil {
	// 		logrus.Fatalf("Failed to sync repo: %v", err)
	// 	}
	// 	logrus.Info("Repo synced")

	// 	repoServices := "./repos/myrepo/services"
	// 	images, err := checker.FindImagesInCompose(repoServices)
	// 	if err != nil {
	// 		logrus.Fatalf("failed to scan compose files: %v", err)
	// 	}

	// 	logrus.Infof("Found %d images:", len(images))
	// 	for _, img := range images {
	// 		parsed := checker.ParseImageName(img.Name)

	// 		digest, err := checker.GetImageDigest(parsed, cfg)
	// 		if err != nil {
	// 			logrus.Warnf("Failed to get digest for %s: %v", parsed.Full, err)
	// 			continue
	// 		}

	// 		logrus.Infof("[%s/%s] digest for %s: %s", img.Host, img.Service, parsed.Full, digest)
	// 	}

	// 	time.Sleep(time.Duration(cfg.Checker.IntervalMinutes) * time.Minute)
	// }

}
