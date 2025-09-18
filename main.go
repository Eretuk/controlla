package main

import (
	"context"
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"controlla/internal/notifier"
	"controlla/internal/registry"
	"controlla/internal/skiplist"
	"controlla/internal/updater"
	"controlla/internal/versionutils"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// UpdateInfo holds all necessary data for an update action.
type UpdateInfo struct {
	RepoPath string
	Services []checker.ServiceInfo
	NewTag   string
}

var commonClients = map[string]*registry.CommonRegistryClient{
	"docker.gitea.com": registry.NewCommonRegistryClient("docker.gitea.com"),
	"quay.io":          registry.NewCommonRegistryClient("quay.io"),
}

// updateCache stores information about found updates.
var updateCache = make(map[string]UpdateInfo)
var cacheMutex sync.Mutex

// For hash-mode needed map hash → {fullImageName, tag}
var callbackHashMap = make(map[string][2]string) // [fullImageName, tag]

// Decode callback data
func decodeCallbackData(data, prefix string) (fullImageName, tag string, ok bool) {
	if !strings.HasPrefix(data, prefix+":") {
		return "", "", false
	}
	enc := strings.TrimPrefix(data, prefix+":")
	if strings.HasPrefix(enc, "h:") && len(enc) == 42 { // "h:" + 40 hex
		hash := strings.TrimPrefix(enc, "h:")
		val, ok := callbackHashMap[hash]
		if !ok {
			return "", "", false
		}
		return val[0], val[1], true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, appConfig *config.AppConfig, sl *skiplist.SkipList) {
	callbackData := callback.Data

	if strings.HasPrefix(callbackData, "update_to:") {
		fullImageName, newTag, ok := decodeCallbackData(callbackData, "update_to")
		if !ok {
			logrus.Errorf("Invalid update callback data format: %s", callbackData)
			return
		}

		logrus.Infof("Received update request for image: %s, tag: %s", fullImageName, newTag)

		// Looking for servies to update
		repoPath := appConfig.Git.LocalRepoPath
		relevantFiles, err := checker.FindRelevantFiles(repoPath)
		if err != nil {
			logrus.Errorf("Error finding relevant files: %v", err)
			notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Error finding relevant files: %v", err))
			return
		}
		services, err := checker.ParseRelevantFiles(relevantFiles)
		if err != nil {
			logrus.Errorf("Error parsing images: %v", err)
			notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Error parsing images: %v", err))
			return
		}

		var servicesToUpdate []checker.ServiceInfo
		for _, service := range services {
			if registry.GetFullImageName(&service.ImageInfo) == fullImageName {
				servicesToUpdate = append(servicesToUpdate, service)
			}
		}

		if len(servicesToUpdate) > 0 {
			// Update all found services
			logrus.Infof("Found %d services to update for image %s", len(servicesToUpdate), fullImageName)
			for _, service := range servicesToUpdate {
				// Проверяем, совпадает ли уже тег
				if service.ImageInfo.Tag == newTag {
					logrus.Infof("Service %s in file %s already has tag %s, skipping update.", service.ServiceName, service.FilePath, newTag)
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("ℹ️ Service %s already has version %s, no update needed.", service.ServiceName, newTag))
					continue
				}
				err := updater.UpdateAndPushService(repoPath, service.FilePath, service.ServiceName, newTag, appConfig.Git.SSHKeyPath)
				if err != nil {
					logrus.Errorf("Error updating service %s: %v", service.ServiceName, err)
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Error updating service %s: %v", service.ServiceName, err))
				} else {
					logrus.Infof("Update successful!")
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("✅ Update successful for service %s to version %s!", service.ServiceName, newTag))
				}
			}
		}
	} else if strings.HasPrefix(callbackData, "ignore:") {
		fullImageName, tagToIgnore, ok := decodeCallbackData(callbackData, "ignore")
		if !ok {
			logrus.Errorf("Invalid ignore callback data format: %s", callbackData)
			return
		}

		logrus.Infof("Received ignore request for image: %s, version: %s", fullImageName, tagToIgnore)

		err := sl.Add(fullImageName, tagToIgnore)
		if err != nil {
			logrus.Errorf("Error adding update to skip list: %v", err)
		} else {
			logrus.Infof("Update for %s with tag %s has been added to the skip list.", fullImageName, tagToIgnore)
		}
	}
	// Send confirmation to user and delete original message
	deleteConfig := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	_, err := bot.Request(deleteConfig)
	if err != nil {
		logrus.Errorf("Error deleting message: %v", err)
	}
}

// Main entry point for the controlla application.
// This service checks container images for updates and notifies via Telegram.
func main() {
	// Set up logging format and level
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	// Load application configuration from YAML file
	appConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading application configuration: %v", err)
	}
	logrus.Infof("Configuration loaded successfully.")

	// Initialize and load skip list for ignored updates
	sl := skiplist.NewSkipList(appConfig.Checker.SkipListFile)
	if err := sl.Load(); err != nil {
		logrus.Errorf("Error loading skip list: %v", err)
	} else {
		logrus.Infof("Skip list loaded successfully.")
	}

	// Remove old repository clone if exists
	if _, err := os.Stat(appConfig.Git.LocalRepoPath); err == nil {
		logrus.Infof("Removing old repository clone...")
		if err := os.RemoveAll(appConfig.Git.LocalRepoPath); err != nil {
			logrus.Errorf("Error removing old repository: %v", err)
		}
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logrus.Infof("Shutting down...")
		cancel()
	}()

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Start Telegram listener in a separate goroutine
	go func() {
		logrus.Infof("Telegram listener started...")
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates := bot.GetUpdatesChan(u)

		for update := range updates {
			if update.CallbackQuery != nil {
				handleCallbackQuery(bot, update.CallbackQuery, appConfig, sl)
			}
			if update.Message != nil && strings.HasPrefix(update.Message.Text, "/ignore_image") {
				args := strings.Fields(update.Message.Text)
				if len(args) != 2 {
					notifier.SendTelegramMessage(bot, update.Message.Chat.ID, "Usage: /ignore_image <full_image_name>")
				} else {
					image := args[1]
					err := sl.AddIgnoredImage(image)
					if err != nil {
						notifier.SendTelegramMessage(bot, update.Message.Chat.ID, fmt.Sprintf("❌ Error adding ignored image: %v", err))
					} else {
						notifier.SendTelegramMessage(bot, update.Message.Chat.ID, fmt.Sprintf("✅ Image %s added to ignored list.", image))
					}
				}
			}
		}
		logrus.Infof("Telegram listener is shutting down.")
	}()

	// Set up periodic check interval
	checkInterval := time.Duration(appConfig.Checker.IntervalMinutes) * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	// Initizle registry clients
	dockerHubClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
	ghcrClient := registry.NewGhcrClient()

	runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient)

	// Main loop: periodic checks
	for {
		select {
		case <-ctx.Done():
			logrus.Infof("Shutting down...")
			return
		case <-ticker.C:
			runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient)
		}
	}
}

// runCheck checks all container images for updates and sends notifications if updates are available.
func runCheck(ctx context.Context, appConfig *config.AppConfig, bot *tgbotapi.BotAPI, sl *skiplist.SkipList, dockerHubClient *registry.DockerHubClient, ghcrClient *registry.GhcrClient) {
	logrus.Infof("Starting image update check...")

	repoPath := appConfig.Git.LocalRepoPath

	if _, err := os.Stat(appConfig.Git.SSHKeyPath); os.IsNotExist(err) {
		logrus.Warnf("🚨 The SSH key file does not exist at path: %s", appConfig.Git.SSHKeyPath)
		chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
		notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("❌ Error: SSH key file not found at path: %s", appConfig.Git.SSHKeyPath))
		return
	}

	err := gitrepo.CloneOrPull(appConfig.Git.RepoURL, appConfig.Git.Branch, repoPath, appConfig.Git.SSHKeyPath)
	if err != nil {
		logrus.Errorf("Error cloning or pulling repository: %v", err)
		chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
		notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("❌ Error cloning/pulling repository: %v", err))
		return
	}
	logrus.Infof("Repository cloned or pulled successfully.")

	// Get list of relevant files in the repository
	relevantFiles, err := checker.FindRelevantFiles(repoPath)
	if err != nil {
		logrus.Errorf("Error finding relevant files: %v", err)
		return
	}
	logrus.Infof("Found %d relevant files.", len(relevantFiles))

	// Parse relevant files to extract services and their images
	services, err := checker.ParseRelevantFiles(relevantFiles)
	if err != nil {
		logrus.Errorf("Error parsing images: %v", err)
		return
	}
	logrus.Infof("Found a total of %d services.", len(services))

	// Group services by full image name to avoid duplicate checks
	groupedServices := make(map[string][]checker.ServiceInfo)
	for _, service := range services {
		fullImageName := registry.GetFullImageName(&service.ImageInfo)
		groupedServices[fullImageName] = append(groupedServices[fullImageName], service)
	}
	logrus.Infof("Found a total of %d unique images", len(groupedServices))

	// Check each unique image for updates
	for fullImageName, services := range groupedServices {
		select {
		case <-ctx.Done():
			return
		default:
			logrus.Infof("Parsing image: Registry=%s, Image=%s, Tag=%s", services[0].ImageInfo.Registry, services[0].ImageInfo.Image, services[0].ImageInfo.Tag)

			if sl.IsSkipped(fullImageName, services[0].ImageInfo.Tag) {
				logrus.Infof("Skipping update for %s. It's in the skip list.", fullImageName)
				continue
			}

			if sl.IsImageIgnored(fullImageName) {
				logrus.Infof("Image %s is globally ignored (skipped.yaml), skipping.", fullImageName)
				continue
			}

			var tags []string
			var err error

			switch services[0].ImageInfo.Registry {
			case "docker.io":
				tags, err = dockerHubClient.GetTags(services[0].ImageInfo.Image)
			case "ghcr.io":
				tags, err = ghcrClient.GetTags(services[0].ImageInfo.Image)
			default:
				client, ok := commonClients[services[0].ImageInfo.Registry]
				if ok {
					tags, err = client.GetTags(services[0].ImageInfo.Image)
				} else {
					logrus.Infof("Skipping unsupported registry: %s", services[0].ImageInfo.Registry)
					continue
				}
			}

			logrus.Debugf("Avaliable tags for %s: %v", fullImageName, tags)

			if err != nil {
				logrus.Errorf("Error getting tags for %s: %v", fullImageName, err)
				continue
			}

			// Filter and sort tags according to SemVer and propose N latest
			latestTags := versionutils.SortAndFilterTags(tags, appConfig.Checker.ProposedTagsCount, appConfig.Checker.IgnoreKeywords)
			if len(latestTags) == 0 {
				logrus.Warnf("Can't check updates for %s. No SemVer tags found. Available tags: %v", fullImageName, tags)

				currentDigest, err1 := getDigestForImage(
					services[0].ImageInfo.Registry,
					services[0].ImageInfo.Image,
					services[0].ImageInfo.Tag,
					dockerHubClient,
					ghcrClient,
				)
				latestDigest, err2 := getDigestForImage(
					services[0].ImageInfo.Registry,
					services[0].ImageInfo.Image,
					"latest",
					dockerHubClient,
					ghcrClient,
				)

				logrus.Debugf("Digest fallback for %s: currentDigest=%s (err=%v), latestDigest=%s (err=%v)",
					fullImageName, currentDigest, err1, latestDigest, err2)

				if err1 == nil && err2 == nil && currentDigest != "" && latestDigest != "" {
					if currentDigest != latestDigest {
						logrus.Warnf("Digest update for %s. Current tag: %s, 'latest' tag digest differs!", fullImageName, services[0].ImageInfo.Tag)
					} else {
						logrus.Infof("%s is actual by digest. Current tag: %s", fullImageName, services[0].ImageInfo.Tag)
					}
				} else {
					logrus.Errorf("Can't check updates for %s. No SemVer tags and no digest fallback.", fullImageName)
					chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
					notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("Can't check updates for %s. No SemVer tags and no digest fallback.", fullImageName))
				}
				continue
			}

			// Check the latest tag is newer than the current tag
			latestAvailable := latestTags[0]
			currentVersion, err := semver.NewVersion(services[0].ImageInfo.Tag)
			if err != nil {
				logrus.Infof("⚠️ Can't check updates for %s. Current tag: %s, it is not use SemVer template.", fullImageName, services[0].ImageInfo.Tag)
				chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
				notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("⚠️ Can't check updates for %s. Current tag: %s, it is not use SemVer template.", fullImageName, services[0].ImageInfo.Tag))
				continue
			}
			latestVersion, _ := semver.NewVersion(latestAvailable)

			if latestVersion.GreaterThan(currentVersion) {
				for _, tag := range latestTags {
					if sl.IsSkipped(fullImageName, tag) {
						logrus.Infof("Version %s for %s is in the skip list. Skipping notification.", tag, fullImageName)
						continue
					}
					// Don't notify if the tag is already in use
					if tag == services[0].ImageInfo.Tag {
						logrus.Infof("Tag %s for %s is already in use. Skipping notification.", tag, fullImageName)
						continue
					}
					logrus.Infof("🚨 New update for %s. Current tag: %s, Available: %s", fullImageName, services[0].ImageInfo.Tag, tag)
					// Generate unique ID for this update
					// Using timestamp for simplicity; in production, consider a more robust method
					// to avoid collisions.
					id := fmt.Sprintf("%x", time.Now().UnixNano())

					// Save update info in cache
					cacheMutex.Lock()
					updateCache[id] = UpdateInfo{
						RepoPath: appConfig.Git.LocalRepoPath,
						Services: services,
						NewTag:   tag,
					}
					cacheMutex.Unlock()

					message := fmt.Sprintf(
						"New update for `%s`. \n"+
							"Current version: `%s`. \n"+
							"Avaliable version: `%s`.",
						fullImageName,
						services[0].ImageInfo.Tag,
						tag,
					)
					// Before send message — encode callback data
					callbackData := encodeCallbackData("update_to", fullImageName, tag)
					// If hash-mode, save to map
					if strings.Contains(callbackData, ":h:") {
						hash := strings.Split(callbackData, ":h:")[1]
						callbackHashMap[hash] = [2]string{fullImageName, tag}
					}
					notifier.SendInteractiveMessage(bot, appConfig.TelegramChatID, message, []string{tag}, callbackData)
				}
			} else {
				logrus.Infof("✅ %s is actual. Current tag: %s", fullImageName, services[0].ImageInfo.Tag)
			}
		}
	}
	logrus.Info("Image check completed.")
}

// Encode callback data
// If the length exceeds 64 characters, fallback to sha1 hash.
func encodeCallbackData(prefix, fullImageName, tag string) string {
	raw := fullImageName + ":" + tag
	enc := base64.RawURLEncoding.EncodeToString([]byte(raw))
	data := prefix + ":" + enc
	if len(data) <= 64 {
		return data
	}
	// fallback на sha1
	hash := sha1.Sum([]byte(raw))
	return prefix + ":h:" + hex.EncodeToString(hash[:])
}

// Get image digest for given registry, image and tag
func getDigestForImage(registryName, image, tag string, dockerHubClient *registry.DockerHubClient, ghcrClient *registry.GhcrClient) (string, error) {
	switch registryName {
	case "docker.io":
		return dockerHubClient.GetDigest(image, tag)
	case "ghcr.io":
		return ghcrClient.GetDigest(image, tag)
	default:
		client, ok := commonClients[registryName]
		if ok {
			return client.GetDigest(image, tag)
		}
	}
	return "", fmt.Errorf("unsupported registry for digest: %s", registryName)
}
