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

// updateCache stores information about found updates.
var updateCache = make(map[string]UpdateInfo)
var cacheMutex sync.Mutex

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
	log.Println("Configuration loaded successfully.")

	// Initialize and load skip list for ignored updates
	sl := skiplist.NewSkipList(appConfig.Checker.SkipListFile)
	if err := sl.Load(); err != nil {
		log.Printf("Error loading skip list: %v", err)
	} else {
		log.Println("Skip list loaded successfully.")
	}

	// Remove old repository clone if exists
	if _, err := os.Stat(appConfig.Git.LocalRepoPath); err == nil {
		log.Println("Removing old repository clone...")
		if err := os.RemoveAll(appConfig.Git.LocalRepoPath); err != nil {
			log.Fatalf("Error removing old repository: %v", err)
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
		log.Println("Shutting down...")
		cancel()
	}()

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Start Telegram listener in a separate goroutine
	go func() {
		log.Println("Telegram listener started...")
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates := bot.GetUpdatesChan(u)

		for update := range updates {
			if update.CallbackQuery != nil {
				handleCallbackQuery(bot, update.CallbackQuery, appConfig, sl)
			}
		}
		log.Println("Telegram listener is shutting down.")
	}()

	// Set up periodic check interval
	checkInterval := time.Duration(appConfig.Checker.IntervalMinutes) * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	// 🚨 НОВОЕ ИСПРАВЛЕНИЕ: Инициализируем клиенты для реестров
	dockerHubClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
	ghcrClient := registry.NewGhcrClient()

	runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient)

	// Main loop: periodic checks
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return
		case <-ticker.C:
			runCheck(ctx, appConfig, bot, sl, dockerHubClient, ghcrClient)
		}
	}
}

// runCheck checks all container images for updates and sends notifications if updates are available.
func runCheck(ctx context.Context, appConfig *config.AppConfig, bot *tgbotapi.BotAPI, sl *skiplist.SkipList, dockerHubClient *registry.DockerHubClient, ghcrClient *registry.GhcrClient) {
	log.Println("Starting image update check...")

	repoPath := appConfig.Git.LocalRepoPath

	if _, err := os.Stat(appConfig.Git.SSHKeyPath); os.IsNotExist(err) {
		log.Printf("🚨 The SSH key file does not exist at path: %s", appConfig.Git.SSHKeyPath)
		// 🚨 ИСПРАВЛЕНИЕ: Преобразуем chatID в int64.
		chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
		notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("❌ Error: SSH key file not found at path: %s", appConfig.Git.SSHKeyPath))
		return
	}

	// 🚨 ИСПРАВЛЕНИЕ: Исправлен вызов CloneOrPull. Эта функция возвращает только 1 значение (error).
	err := gitrepo.CloneOrPull(appConfig.Git.RepoURL, appConfig.Git.Branch, repoPath, appConfig.Git.SSHKeyPath)
	if err != nil {
		log.Printf("Error cloning or pulling repository: %v", err)
		// 🚨 ИСПРАВЛЕНИЕ: Преобразуем chatID в int64.
		chatID, _ := strconv.ParseInt(appConfig.TelegramChatID, 10, 64)
		notifier.SendTelegramMessage(bot, chatID, fmt.Sprintf("❌ Error cloning/pulling repository: %v", err))
		return
	}
	log.Println("Repository cloned or pulled successfully.")

	// Получаем все .yml и .yaml файлы
	relevantFiles, err := checker.FindRelevantFiles(repoPath)
	if err != nil {
		log.Printf("Error finding relevant files: %v", err)
		return
	}
	log.Printf("Found %d relevant files.", len(relevantFiles))

	// Парсим образы из найденных файлов
	services, err := checker.ParseRelevantFiles(relevantFiles)
	if err != nil {
		log.Printf("Error parsing images: %v", err)
		return
	}
	log.Printf("Found a total of %d services.", len(services))

	// Группируем сервисы по образу
	groupedServices := make(map[string][]checker.ServiceInfo)
	for _, service := range services {
		fullImageName := registry.GetFullImageName(&service.ImageInfo)
		groupedServices[fullImageName] = append(groupedServices[fullImageName], service)
	}
	log.Printf("Found a total of %d unique images", len(groupedServices))

	// Проверяем обновления для каждого уникального образа
	for fullImageName, services := range groupedServices {
		select {
		case <-ctx.Done():
			return
		default:
			log.Printf("Parsing image: Registry=%s, Image=%s, Tag=%s", services[0].ImageInfo.Registry, services[0].ImageInfo.Image, services[0].ImageInfo.Tag)

			if sl.IsSkipped(fullImageName, services[0].ImageInfo.Tag) {
				log.Printf("Skipping update for %s. It's in the skip list.", fullImageName)
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
				log.Printf("Skipping unsupported registry: %s", services[0].ImageInfo.Registry)
				// curl -s "https://docker.gitea.com/v2/gitea/tags/list"
				continue
			}

			if err != nil {
				log.Printf("Error getting tags for %s: %v", fullImageName, err)
				continue
			}

			// Фильтруем теги по SemVer и получаем N последних
			latestTags := versionutils.GetLatestTags(tags, appConfig.Checker.ProposedTagsCount)
			if len(latestTags) == 0 {
				log.Printf("⚠️ Can't check updates for %s. No SemVer tags found.", fullImageName)
				continue
			}

			// Сравниваем версии
			latestAvailable := latestTags[0]
			currentVersion, err := semver.NewVersion(services[0].ImageInfo.Tag)
			if err != nil {
				log.Printf("⚠️ Can't check updates for %s. Current tag: %s, it is not use SemVer template.", fullImageName, services[0].ImageInfo.Tag)
				continue
			}
			latestVersion, _ := semver.NewVersion(latestAvailable) // Ошибку игнорируем, т.к. тег уже отфильтрован

			if latestVersion.GreaterThan(currentVersion) {
				for _, tag := range latestTags {
					if sl.IsSkipped(fullImageName, tag) {
						log.Printf("Version %s for %s is in the skip list. Skipping notification.", tag, fullImageName)
						continue
					}
					// Don't notify if the tag is already in use
					if tag == services[0].ImageInfo.Tag {
						log.Printf("Tag %s for %s is already in use. Skipping notification.", tag, fullImageName)
						continue
					}
					log.Printf("🚨 New update for %s. Current tag: %s, Available: %s", fullImageName, services[0].ImageInfo.Tag, tag)
					// Генерируем уникальный ID для кеша
					id := fmt.Sprintf("%x", time.Now().UnixNano())

					// Сохраняем информацию об обновлении в кеш
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
					notifier.SendInteractiveMessage(bot, appConfig.TelegramChatID, message, []string{tag}, fullImageName)
				}
			} else {
				log.Printf("✅ %s is actual. Current tag: %s", fullImageName, services[0].ImageInfo.Tag)
			}
		}
	}
	log.Println("Image check completed.")
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, appConfig *config.AppConfig, sl *skiplist.SkipList) {
	callbackData := callback.Data

	if strings.HasPrefix(callbackData, "update_to:") {
		// Парсим callback data, чтобы получить имя образа и новый тег
		parts := strings.Split(callbackData, ":")
		if len(parts) != 3 {
			log.Printf("Invalid update callback data format: %s", callbackData)
			return
		}
		fullImageName := parts[1]
		newTag := parts[2]

		log.Printf("Received update request for image: %s, tag: %s", fullImageName, newTag)

		// 🚨 Новый способ: ищем сервисы прямо сейчас в репозитории
		repoPath := appConfig.Git.LocalRepoPath
		relevantFiles, err := checker.FindRelevantFiles(repoPath)
		if err != nil {
			log.Printf("Error finding relevant files: %v", err)
			notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Error finding relevant files: %v", err))
			return
		}
		services, err := checker.ParseRelevantFiles(relevantFiles)
		if err != nil {
			log.Printf("Error parsing images: %v", err)
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
			// Обновляем все найденные сервисы
			for _, service := range servicesToUpdate {
				// Проверяем, совпадает ли уже тег
				if service.ImageInfo.Tag == newTag {
					log.Printf("Service %s in file %s already has tag %s, skipping update.", service.ServiceName, service.FilePath, newTag)
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("ℹ️ Service %s already has version %s, no update needed.", service.ServiceName, newTag))
					continue
				}
				err := updater.UpdateAndPushService(repoPath, service.FilePath, service.ServiceName, newTag, appConfig.Git.SSHKeyPath)
				if err != nil {
					log.Printf("Error updating service %s: %v", service.ServiceName, err)
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Error updating service %s: %v", service.ServiceName, err))
				} else {
					log.Println("Update successful!")
					notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("✅ Update successful for service %s to version %s!", service.ServiceName, newTag))
				}
			}
		}
	} else if strings.HasPrefix(callbackData, "ignore:") {
		// Парсим callback data, чтобы получить имя образа и тег для игнорирования
		parts := strings.Split(callbackData, ":")
		if len(parts) != 3 {
			log.Printf("Invalid ignore callback data format: %s", callbackData)
			return
		}
		fullImageName := parts[1]
		tagToIgnore := parts[2]

		log.Printf("Received ignore request for image: %s, version: %s", fullImageName, tagToIgnore)

		err := sl.Add(fullImageName, tagToIgnore)
		if err != nil {
			log.Printf("Error adding update to skip list: %v", err)
		} else {
			log.Printf("Update for %s with tag %s has been added to the skip list.", fullImageName, tagToIgnore)
		}
	}
	// Отправляем пользователю подтверждение и удаляем исходное сообщение
	deleteConfig := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	_, err := bot.Request(deleteConfig)
	if err != nil {
		log.Printf("Error deleting message: %v", err)
	}
}
