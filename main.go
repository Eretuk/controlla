package main

import (
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"controlla/internal/notifier"
	"controlla/internal/registry"
	"controlla/internal/versionutils"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
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
	// for _, file := range foundFiles {
	// 	log.Println(file)
	// }

	var allImages []string

	// Iterate through the list of found files and parse each one.
	log.Printf("Parsing relevant files for images...")
	for _, file := range foundFiles {
		imagesInFile, err := checker.FindImagesFromCompose(file)
		if err != nil {
			log.Printf("Warning: Failed to parse images from %s: %v", file, err)
			continue // Continue to the next file if one fails.
		}

		allImages = append(allImages, imagesInFile...)
	}
	// Now, you can remove duplicates and then check each image against Docker Hub.
	uniqueImages := make(map[string]bool)
	var finalImageList []string
	for _, image := range allImages {
		if _, exists := uniqueImages[image]; !exists {
			uniqueImages[image] = true
			finalImageList = append(finalImageList, image)
		}
	}
	// log.Printf("Found a total of %d unique images: %v", len(finalImageList), finalImageList)
	log.Printf("Found a total of %d unique images", len(finalImageList))

	// Step 3: Iterate through the list of unique images and parse each one.
	for _, imageString := range finalImageList {
		parsedImage := checker.ParseDockerImage(imageString)

		// Build the log message dynamically
		logMsg := "Parsed image: Registry=%s, Image=%s, Tag=%s"
		logArgs := []interface{}{parsedImage.Registry, parsedImage.Image, parsedImage.Tag}

		if parsedImage.Digest != "" {
			logMsg += ", Digest=%s"
			logArgs = append(logArgs, parsedImage.Digest)
		}

		log.Printf(logMsg, logArgs...)
		if parsedImage.Registry == "docker.io" {
			dockerClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
			tags, err := dockerClient.GetTags(parsedImage.Image)
			if err != nil {
				log.Printf("Error getting Docker Hub tags for %s: %v", parsedImage.Image, err)
				return
			}
			// Add check for empty tags from DockerHub
			if len(tags) == 0 {
				log.Printf("No tags found for image %s", parsedImage.Image)
				continue
			}
			latestTags := versionutils.GetLatestTags(tags, cfg.Checker.ProposedTagsCount)
			log.Printf("Docker Hub tags for %s: %v", parsedImage.Image, latestTags)

			// Parse tag for checks
			_, err = semver.NewVersion(strings.TrimPrefix(parsedImage.Tag, "v"))
			if err != nil {
				log.Printf("⚠️ Can't check updates for %s. Current tag: %s, it is not use SemVer template.", parsedImage.Image, parsedImage.Tag)
				continue
			}

			latestAvailable, isUpdateAvailable := versionutils.CheckForUpdate(parsedImage.Tag, latestTags)
			if isUpdateAvailable {
				log.Printf("🚨 New update for %s. Current tag: %s, Avaliable: %s", parsedImage.Image, parsedImage.Tag, latestAvailable)
				log.Printf("Send notification to telegram")
				message := fmt.Sprintf("🚨 New update for %s\nCurrent version: `%s`\nAvaliable version: `%s`", parsedImage.Image, parsedImage.Tag, latestAvailable)
				notifier.SendTelegramMessage(appConfig.TelegramToken, appConfig.TelegramChatID, message)
			} else {
				log.Printf("✅ %s is actual. Current tag: %s", parsedImage.Image, parsedImage.Tag)
			}
		} else {
			// log.Printf("Skipping image %s: registry %s is not yet supported.", imageString, parsedImage.Registry)
		}
	}
	// logrus.Infof("Controlla started. Checking every %d minutes.", cfg.Checker.IntervalMinutes)

	// 2. Create clients
	// ghcrClient := registry.NewGhcrClient(appConfig.GithubToken)

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
