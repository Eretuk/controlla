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
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// UpdateInfo holds all necessary data for an update action.
type UpdateInfo struct {
	RepoPath string
	Services []checker.ServiceInfo
	NewTag   string
}

// updateCache stores information about found updates.
var updateCache = make(map[string]UpdateInfo)
var cacheMutex sync.Mutex

// sentNotifications stores a record of notifications already sent to Telegram
// to prevent sending duplicates for the same image/tag combination.
var sentNotifications = make(map[string]bool)
var sentNotificationsMutex sync.Mutex

// For hash-mode: maps sha1 hash → {fullImageName, tag}
var callbackHashMap = make(map[string][2]string)

func decodeCallbackData(data, prefix string) (fullImageName, tag string, ok bool) {
	if !strings.HasPrefix(data, prefix+":") {
		return "", "", false
	}
	enc := strings.TrimPrefix(data, prefix+":")
	if strings.HasPrefix(enc, "h:") && len(enc) == 42 { // "h:" + 40 hex chars
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

		sentNotificationsMutex.Lock()
		delete(sentNotifications, fullImageName+":"+newTag)
		sentNotificationsMutex.Unlock()

		repoPath := appConfig.Git.LocalRepoPath
		relevantFiles, err := checker.FindRelevantFiles(repoPath, appConfig.Checker.WatchFolder)
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
			logrus.Infof("Found %d services to update for image %s", len(servicesToUpdate), fullImageName)
			for _, service := range servicesToUpdate {
				if service.ImageInfo.Tag == newTag {
					logrus.Infof("Service %s in file %s already has tag %s, skipping update.", service.ServiceName, service.FilePath, newTag)
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("ℹ️ Service %s already has version %s, no update needed.", service.ServiceName, newTag))
					continue
				}
				err := updater.UpdateAndPushService(
					repoPath,
					service.FilePath,
					service.ServiceName,
					newTag,
					appConfig.Git.SSHKeyPath,
					appConfig.Git.AuthorName,
					appConfig.Git.AuthorEmail,
				)
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

		sentNotificationsMutex.Lock()
		delete(sentNotifications, fullImageName+":"+tagToIgnore)
		sentNotificationsMutex.Unlock()

		err := sl.Add(fullImageName, tagToIgnore)
		if err != nil {
			logrus.Errorf("Error adding update to skip list: %v", err)
		} else {
			logrus.Infof("Update for %s with tag %s has been added to the skip list.", fullImageName, tagToIgnore)
		}
	}

	// Clear all cached notifications for this image after any user action.
	var fullImageName string
	if strings.HasPrefix(callbackData, "update_to:") {
		fullImageName, _, _ = decodeCallbackData(callbackData, "update_to")
	} else if strings.HasPrefix(callbackData, "ignore:") {
		fullImageName, _, _ = decodeCallbackData(callbackData, "ignore")
	}
	if fullImageName != "" {
		sentNotificationsMutex.Lock()
		for key := range sentNotifications {
			if strings.HasPrefix(key, fullImageName+":") {
				delete(sentNotifications, key)
			}
		}
		sentNotificationsMutex.Unlock()
		logrus.Debugf("Cleared all pending notifications for %s after user action.", fullImageName)
	}

	// Delete the original Telegram message.
	deleteConfig := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	_, err := bot.Request(deleteConfig)
	if err != nil {
		logrus.Errorf("Error deleting message: %v", err)
	}
}

func main() {
	configPath := flag.String("config-file", "./data/config.yaml", "Path to config file")
	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	appConfig, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading application configuration: %v", err)
	}
	logrus.Infof("Configuration loaded successfully.")

	sl := skiplist.NewSkipList(appConfig.Checker.SkipListFile)
	if err := sl.Load(); err != nil {
		logrus.Errorf("Error loading skip list: %v", err)
	} else {
		logrus.Infof("Skip list loaded successfully.")
	}

	// Remove old repository clone if exists.
	if _, err := os.Stat(appConfig.Git.LocalRepoPath); err == nil {
		logrus.Infof("Removing old repository clone...")
		if err := os.RemoveAll(appConfig.Git.LocalRepoPath); err != nil {
			logrus.Errorf("Error removing old repository: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Infof("Shutting down...")
		cancel()
	}()

	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

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

	checkInterval := time.Duration(appConfig.Checker.IntervalMinutes) * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	dockerHubClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
	ghcrClient := registry.NewGhcrClient(appConfig.GithubToken)
	commonClients := make(map[string]*registry.CommonRegistryClient)

	runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient, commonClients)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient, commonClients)
		}
	}
}

// runCheck checks all container images for updates and sends Telegram notifications.
func runCheck(ctx context.Context, appConfig *config.AppConfig, bot *tgbotapi.BotAPI, sl *skiplist.SkipList, dockerHubClient *registry.DockerHubClient, ghcrClient *registry.GhcrClient, commonClients map[string]*registry.CommonRegistryClient) {
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

	relevantFiles, err := checker.FindRelevantFiles(repoPath, appConfig.Checker.WatchFolder)
	if err != nil {
		logrus.Errorf("Error finding relevant files: %v", err)
		return
	}
	logrus.Infof("Found %d relevant files in watch folder '%s'.", len(relevantFiles), appConfig.Checker.WatchFolder)

	services, err := checker.ParseRelevantFiles(relevantFiles)
	if err != nil {
		logrus.Errorf("Error parsing images: %v", err)
		return
	}
	logrus.Infof("Found a total of %d services.", len(services))

	// Group services by full image name to avoid duplicate registry calls.
	groupedServices := make(map[string][]checker.ServiceInfo)
	for _, service := range services {
		fullImageName := registry.GetFullImageName(&service.ImageInfo)
		groupedServices[fullImageName] = append(groupedServices[fullImageName], service)
	}
	logrus.Infof("Found a total of %d unique images", len(groupedServices))

	// A small sleep between requests reduces burst pressure on registries.
	const interRequestDelay = 2 * time.Second

	for fullImageName, services := range groupedServices {
		select {
		case <-ctx.Done():
			return
		default:
		}

		time.Sleep(interRequestDelay)

		currentImageTag := services[0].ImageInfo.Tag
		logrus.Infof("Parsing image: Registry=%s, Image=%s, Tag=%s", services[0].ImageInfo.Registry, services[0].ImageInfo.Image, currentImageTag)

		// Skip images that use the "latest" tag — there is no meaningful version
		// to compare against, so checking them just wastes registry quota.
		if currentImageTag == "latest" {
			logrus.Infof("Skipping %s: tag is 'latest', version tracking is not possible.", fullImageName)
			continue
		}

		// Skip images on the skip/ignore list.
		if sl.IsSkipped(fullImageName, currentImageTag) || sl.IsImageIgnored(fullImageName) {
			if sl.IsSkipped(fullImageName, currentImageTag) {
				logrus.Infof("Skipping update for %s. It's in the skip list.", fullImageName)
			} else {
				logrus.Infof("Image %s is globally ignored (skipped.yaml), skipping.", fullImageName)
			}
			sentNotificationsMutex.Lock()
			for key := range sentNotifications {
				if strings.HasPrefix(key, fullImageName+":") {
					delete(sentNotifications, key)
				}
			}
			sentNotificationsMutex.Unlock()
			continue
		}

		var tags []string
		registryName := services[0].ImageInfo.Registry

		switch registryName {
		case "docker.io":
			tags, err = dockerHubClient.GetTags(services[0].ImageInfo.Image)
		case "ghcr.io":
			tags, err = ghcrClient.GetTags(services[0].ImageInfo.Image, appConfig.Checker.GhcrMaxPages)
		default:
			client, ok := commonClients[registryName]
			if !ok {
				logrus.Debugf("Creating new common client for registry: %s", registryName)
				client = registry.NewCommonRegistryClient(registryName)
				commonClients[registryName] = client
			}
			tags, err = client.GetTags(services[0].ImageInfo.Image)
		}

		logrus.Debugf("Available tags for %s: %v", fullImageName, tags)

		if err != nil {
			logrus.Errorf("Error getting tags for %s: %v", fullImageName, err)
			notificationKey := fullImageName + ":error_getting_tags"
			sentNotificationsMutex.Lock()
			if sentNotifications[notificationKey] {
				sentNotificationsMutex.Unlock()
				continue
			}
			sentNotifications[notificationKey] = true
			sentNotificationsMutex.Unlock()
			chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
			notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("❌ Error getting tags for %s: %v", fullImageName, err))
			continue
		}

		// Successful fetch — clear any previously cached error notification so it
		// can fire again if the image breaks in a future cycle.
		sentNotificationsMutex.Lock()
		delete(sentNotifications, fullImageName+":error_getting_tags")
		sentNotificationsMutex.Unlock()

		// Filter and sort tags by SemVer, return the top N candidates.
		latestTags := versionutils.SortAndFilterTags(tags, appConfig.Checker.ProposedTagsCount, appConfig.Checker.IgnoreKeywords)
		if len(latestTags) == 0 {
			logrus.Warnf("⚠️ Can't check updates for %s. Current tag: %s is not a SemVer tag. Available tags: %v",
				fullImageName, currentImageTag, tags)

			currentDigest, err1 := getDigestForImage(services[0].ImageInfo.Registry, services[0].ImageInfo.Image, services[0].ImageInfo.Tag, dockerHubClient, ghcrClient, commonClients)
			latestDigest, err2 := getDigestForImage(services[0].ImageInfo.Registry, services[0].ImageInfo.Image, "latest", dockerHubClient, ghcrClient, commonClients)

			logrus.Debugf("Digest fallback for %s: currentDigest=%s (err=%v), latestDigest=%s (err=%v)",
				fullImageName, currentDigest, err1, latestDigest, err2)

			if err1 == nil && err2 == nil && currentDigest != "" && latestDigest != "" {
				if currentDigest != latestDigest {
					logrus.Warnf("🚨 Digest update for %s. Current tag: %s, 'latest' tag digest differs!", fullImageName, currentImageTag)
					notificationKey := fullImageName + ":digest_update"
					sentNotificationsMutex.Lock()
					if sentNotifications[notificationKey] {
						sentNotificationsMutex.Unlock()
						continue
					}
					sentNotifications[notificationKey] = true
					sentNotificationsMutex.Unlock()
					chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
					notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("🚨 Digest update for %s. Current tag: %s, 'latest' tag digest differs!", fullImageName, currentImageTag))
				} else {
					logrus.Infof("%s is up to date by digest. Current tag: %s", fullImageName, currentImageTag)
				}
			} else {
				logrus.Errorf("Can't check updates for %s. No SemVer tags and no digest fallback.", fullImageName)
			}
			continue
		}

		// Compare current tag against the latest available tag.
		latestAvailable := latestTags[0]
		normalizedCurrentTag := versionutils.TryNormalizeTagForSemVer(currentImageTag)
		currentVersion, err := semver.NewVersion(strings.TrimPrefix(normalizedCurrentTag, "v"))
		if err != nil {
			logrus.Warnf("⚠️ Can't parse current tag for %s as SemVer. Tag: %s", fullImageName, currentImageTag)
			notificationKey := fullImageName + ":" + currentImageTag + ":non_semver_warning"
			sentNotificationsMutex.Lock()
			if sentNotifications[notificationKey] {
				sentNotificationsMutex.Unlock()
				continue
			}
			sentNotifications[notificationKey] = true
			sentNotificationsMutex.Unlock()
			chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
			notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("⚠️ Can't check updates for %s. Current tag: %s is not a SemVer tag.", fullImageName, currentImageTag))
			continue
		}
		latestVersion, err := semver.NewVersion(strings.TrimPrefix(versionutils.TryNormalizeTagForSemVer(latestAvailable), "v"))
		if err != nil || latestVersion == nil {
			logrus.Warnf("⚠️ Can't parse latest tag %q for %s as SemVer, skipping.", latestAvailable, fullImageName)
			continue
		}

		if latestVersion.GreaterThan(currentVersion) {
			for _, tag := range latestTags {
				if sl.IsSkipped(fullImageName, tag) {
					logrus.Infof("Version %s for %s is in the skip list. Skipping notification.", tag, fullImageName)
					continue
				}
				if tag == services[0].ImageInfo.Tag {
					logrus.Infof("Tag %s for %s is already in use. Skipping notification.", tag, fullImageName)
					continue
				}

				// Only notify for tags that are strictly newer than the current one.
				tagVersion, err := semver.NewVersion(strings.TrimPrefix(versionutils.TryNormalizeTagForSemVer(tag), "v"))
				if err != nil || tagVersion == nil || !tagVersion.GreaterThan(currentVersion) {
					logrus.Debugf("Tag %s for %s is not newer than current %s, skipping.", tag, fullImageName, currentImageTag)
					continue
				}

				notificationKey := fullImageName + ":" + tag
				sentNotificationsMutex.Lock()
				if sentNotifications[notificationKey] {
					logrus.Infof("Notification for %s with tag %s already sent. Skipping duplicate.", fullImageName, tag)
					sentNotificationsMutex.Unlock()
					continue
				}
				sentNotifications[notificationKey] = true
				sentNotificationsMutex.Unlock()

				logrus.Infof("🚨 New update for %s. Current tag: %s, Available: %s", fullImageName, services[0].ImageInfo.Tag, tag)

				id := fmt.Sprintf("%x", time.Now().UnixNano())
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
						"Available version: `%s`.",
					fullImageName,
					services[0].ImageInfo.Tag,
					tag,
				)
				callbackData := encodeCallbackData("update_to", fullImageName, tag)
				if strings.Contains(callbackData, ":h:") {
					hash := strings.Split(callbackData, ":h:")[1]
					callbackHashMap[hash] = [2]string{fullImageName, tag}
				}
				notifier.SendInteractiveMessage(bot, appConfig.TelegramChatID, message, []string{tag}, callbackData)
			}
			// Clear any stale non-semver warning now that we found valid tags.
			sentNotificationsMutex.Lock()
			delete(sentNotifications, fullImageName+":"+currentImageTag+":non_semver_warning")
			sentNotificationsMutex.Unlock()
		} else {
			logrus.Infof("✅ %s is up to date. Current tag: %s", fullImageName, currentImageTag)
			// Image is current — clear all cached notifications so they can re-fire
			// if something changes later.
			sentNotificationsMutex.Lock()
			for key := range sentNotifications {
				if strings.HasPrefix(key, fullImageName+":") {
					delete(sentNotifications, key)
				}
			}
			sentNotificationsMutex.Unlock()
		}
	}
	logrus.Info("Image check completed.")
}

// encodeCallbackData encodes prefix + image + tag into a Telegram callback string.
// Falls back to a sha1 hash when the encoded value exceeds 64 bytes.
func encodeCallbackData(prefix, fullImageName, tag string) string {
	raw := fullImageName + ":" + tag
	enc := base64.RawURLEncoding.EncodeToString([]byte(raw))
	data := prefix + ":" + enc
	if len(data) <= 64 {
		return data
	}
	hash := sha1.Sum([]byte(raw))
	return prefix + ":h:" + hex.EncodeToString(hash[:])
}

// getDigestForImage returns the manifest digest for a given registry/image/tag.
func getDigestForImage(registryName, image, tag string, dockerHubClient *registry.DockerHubClient, ghcrClient *registry.GhcrClient, commonClients map[string]*registry.CommonRegistryClient) (string, error) {
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
