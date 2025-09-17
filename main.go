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

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	appConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading application configuration: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	if _, err := os.Stat(appConfig.Git.LocalRepoPath); err == nil {
		log.Println("Removing old repository clone...")
		err := os.RemoveAll(appConfig.Git.LocalRepoPath)
		if err != nil {
			log.Printf("Failed to remove old repository: %v", err)
		}
	}

	// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Загружаем список пропущенных обновлений
	skipList := skiplist.NewSkipList(appConfig.Checker.SkipListFile)
	if err := skipList.Load(); err != nil {
		log.Fatalf("Error loading skip list: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot API client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go startTelegramListener(ctx, bot, appConfig, skipList)

	log.Println("Starting image update check...")
	checkUpdates(appConfig, bot, skipList)

	log.Println("Image check completed. Waiting for termination signal...")
	<-sigChan
	log.Println("Shutting down...")
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

// startTelegramListener запускает слушателя для Telegram API.
func startTelegramListener(ctx context.Context, bot *tgbotapi.BotAPI, appConfig *config.AppConfig, skipList *skiplist.SkipList) {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	log.Println("Telegram listener started...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram listener is shutting down.")
			return
		case update := <-updates:
			// Передаем skipList дальше
			if update.CallbackQuery != nil {
				handleCallbackQuery(bot, update.CallbackQuery, appConfig, skipList)
			}
		}
	}
}

// handleCallbackQuery обрабатывает нажатие на inline-кнопку.
func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, appConfig *config.AppConfig, skipList *skiplist.SkipList) {
	callbackData := callback.Data

	if strings.HasPrefix(callbackData, "update_to:") {
		deleteConfig := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
		_, err := bot.Request(deleteConfig)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		}
		// Парсим ID и NewTag из callbackData
		parts := strings.Split(callbackData, ":")
		if len(parts) != 3 {
			log.Printf("Invalid callback data format: %s", callbackData)
			return
		}
		id := parts[1]
		newTag := parts[2]

		// Получаем информацию из кеша
		cacheMutex.Lock()
		info, ok := updateCache[id]
		cacheMutex.Unlock()

		if !ok {
			log.Printf("Error: Update info for ID %s not found in cache. Callback Data: %s", id, callbackData)
			return
		}

		// Отправляем сообщение об обработке
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("Processing update for %s to version %s...", info.Services[0].ImageInfo.Image, newTag))
		msg.ReplyToMessageID = callback.Message.MessageID
		bot.Send(msg)

		// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Перебираем все сервисы, которые используют этот образ
		for _, service := range info.Services {
			log.Printf("Updating service %s in file %s to tag %s", service.ServiceName, service.FilePath, newTag)

			// Вызываем функцию для каждого отдельного сервиса с правильными аргументами
			err := updater.UpdateAndPushService(info.RepoPath, service.FilePath, service.ServiceName, newTag, appConfig.Git.SSHKeyPath)
			if err != nil {
				log.Printf("Error updating service %s: %v", service.ServiceName, err)
				notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("❌ Update failed for service %s: %v", service.ServiceName, err))
				// Мы не завершаем работу, чтобы попытаться обновить остальные сервисы, если они есть
			} else {
				log.Println("Update successful!")
				notifier.SendTelegramMessage(bot, callback.Message.Chat.ID, fmt.Sprintf("✅ Update successful for service %s to version %s!", service.ServiceName, newTag))
			}
		}
	} else if strings.HasPrefix(callbackData, "ignore") {
		id := strings.TrimPrefix(callbackData, "ignore")
		// Получаем информацию из кеша
		cacheMutex.Lock()
		info, ok := updateCache[id]
		cacheMutex.Unlock()
		if ok {
			// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Используем skipList для вызова метода Add
			fullImageName := registry.GetFullImageName(&info.Services[0].ImageInfo)
			err := skipList.Add(fullImageName, info.NewTag)
			if err != nil {
				log.Printf("Error adding update to skip list: %v", err)
			}
		}
		// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Логика для кнопки "Skip"
		log.Printf("Received skip request for update ID: %s", strings.TrimPrefix(callbackData, "skip_update:"))

		// Отправляем пользователю подтверждение и удаляем исходное сообщение
		deleteConfig := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
		_, err := bot.Request(deleteConfig)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		}

		// Дополнительно можно отправить сообщение, что обновление пропущено
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "✅ Update skipped.")
		msg.ReplyToMessageID = callback.Message.MessageID
		bot.Send(msg)
	}
}

func checkUpdates(appConfig *config.AppConfig, bot *tgbotapi.BotAPI, skipList *skiplist.SkipList) {
	// 1. Клонируем или обновляем репозиторий
	log.Printf("Cloning repository %s to %s", appConfig.Git.RepoURL, appConfig.Git.LocalRepoPath)
	err := gitrepo.CloneOrPull(appConfig.Git.RepoURL, appConfig.Git.Branch, appConfig.Git.LocalRepoPath, appConfig.Git.SSHKeyPath)
	if err != nil {
		log.Printf("Failed to clone or pull repository: %v", err)
		return
	}
	log.Println("Repository cloned or pulled successfully.")

	// 2. Находим релевантные файлы
	filePaths, err := checker.FindRelevantFiles(appConfig.Git.LocalRepoPath)
	if err != nil {
		log.Printf("Error finding files: %v", err)
		return
	}
	log.Printf("Found %d relevant files.", len(filePaths))

	// 3. Парсим файлы и получаем полный список сервисов
	allServices, err := checker.ParseRelevantFiles(filePaths)
	if err != nil {
		log.Printf("Error parsing files: %v", err)
		return
	}
	log.Printf("Found a total of %d services.", len(allServices))

	// 4. Группируем сервисы по образу
	servicesByImage := make(map[string][]checker.ServiceInfo)
	for _, service := range allServices {
		fullImageName := fmt.Sprintf("%s/%s", service.ImageInfo.Registry, service.ImageInfo.Image)
		servicesByImage[fullImageName] = append(servicesByImage[fullImageName], service)
	}
	log.Printf("Found a total of %d unique images", len(servicesByImage))

	// 5. Проверяем обновления для каждого уникального образа
	dockerhubClient := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
	for fullImageName, services := range servicesByImage {
		serviceInfo := services[0] // Берем первый сервис из группы для проверки

		log.Printf("Parsing image: Registry=%s, Image=%s, Tag=%s", serviceInfo.ImageInfo.Registry, serviceInfo.ImageInfo.Image, serviceInfo.ImageInfo.Tag)

		// Мы проверяем только образы из docker.io
		if serviceInfo.ImageInfo.Registry != "docker.io" {
			log.Printf("Skipping registry %s for image %s", serviceInfo.ImageInfo.Registry, fullImageName)
			continue
		}

		// Получаем список тегов из Docker Hub
		tags, err := dockerhubClient.GetTags(serviceInfo.ImageInfo.Image)
		if err != nil {
			log.Printf("Error getting tags for %s: %v", serviceInfo.ImageInfo.Image, err)
			continue
		}

		// Фильтруем теги по SemVer
		latestTags := versionutils.GetLatestTags(tags, appConfig.Checker.ProposedTagsCount)
		if len(latestTags) == 0 {
			log.Printf("⚠️ Can't check updates for %s. No SemVer tags found.", fullImageName)
			continue
		}

		// Сравниваем версии
		latestAvailable := latestTags[0]
		currentVersion, err := semver.NewVersion(serviceInfo.ImageInfo.Tag)
		if err != nil {
			log.Printf("⚠️ Can't check updates for %s. Current tag: %s, it is not use SemVer template.", fullImageName, serviceInfo.ImageInfo.Tag)
			continue
		}
		latestVersion, _ := semver.NewVersion(latestAvailable) // Ошибку игнорируем, т.к. тег уже отфильтрован

		if latestVersion.GreaterThan(currentVersion) {
			log.Printf("🚨 New update for %s. Current tag: %s, Available: %s", fullImageName, serviceInfo.ImageInfo.Tag, latestAvailable)

			// Генерируем уникальный ID для кеша
			id := fmt.Sprintf("%x", time.Now().UnixNano())

			// Сохраняем информацию об обновлении в кеш
			cacheMutex.Lock()
			updateCache[id] = UpdateInfo{
				RepoPath: appConfig.Git.LocalRepoPath,
				Services: services,
				NewTag:   latestAvailable,
			}
			cacheMutex.Unlock()

			message := fmt.Sprintf("🚨 New update for %s\nCurrent version: `%s`\nAvaliable version: `%s`", fullImageName, serviceInfo.ImageInfo.Tag, latestAvailable)
			// Отправляем сообщение, передавая ID
			notifier.SendInteractiveMessage(bot, appConfig.TelegramChatID, message, latestTags, id)
		} else {
			log.Printf("✅ %s is actual. Current tag: %s", fullImageName, serviceInfo.ImageInfo.Tag)
		}
	}
}
