package main

import (
	"context"
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"controlla/internal/notifier"
	"controlla/internal/registry"
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

	log.Printf("Loaded Telegram Token: '%s'", appConfig.TelegramToken)
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Error creating Telegram bot API client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go startTelegramListener(ctx, bot)

	log.Println("Starting image update check...")
	checkUpdates(appConfig, bot)

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
func startTelegramListener(ctx context.Context, bot *tgbotapi.BotAPI) {
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
			// Здесь мы будем обрабатывать нажатия на кнопки
			if update.CallbackQuery != nil {
				handleCallbackQuery(bot, update.CallbackQuery)
			}
		}
	}
}

// handleCallbackQuery обрабатывает нажатие на inline-кнопку.
func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	callbackData := callback.Data
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID

	log.Printf("Received callback query: %s from user %s", callbackData, callback.From.UserName)

	// Здесь мы будем парсить данные
	if strings.HasPrefix(callbackData, "update_to:") {
		id := strings.TrimPrefix(callbackData, "update_to:")
		// Получаем информацию из кеша
		cacheMutex.Lock()
		info, ok := updateCache[id]
		cacheMutex.Unlock()
		if !ok {
			log.Printf("Error: Update info for ID %s not found in cache.", id)
			return
		}
		// Вызываем нашу новую функцию
		serviceInfo := info.Services[0]
		err := updater.UpdateAndPushService(info.RepoPath, serviceInfo.FilePath, serviceInfo.ServiceName, info.NewTag)
		if err != nil {
			log.Printf("Error updating and pushing: %v", err)
			// Отправь сообщение об ошибке в Telegram
		} else {
			// Отправь сообщение об успехе
		}
		versionToUpdate := strings.TrimPrefix(callbackData, "update_to:")
		// В будущем, здесь ты будешь запускать скрипт обновления
		log.Printf("Request to update to version %s received. ChatID: %d, MessageID: %d", versionToUpdate, chatID, messageID)

		// Отправляем ответ, чтобы убрать "часики" с кнопки
		answerCallback := tgbotapi.NewCallback(callback.ID, "Обновление запущено!")
		bot.Send(answerCallback)

		// Отправляем новое сообщение, подтверждающее действие
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Начинаю обновление до версии `%s`...", versionToUpdate))
		msg.ParseMode = tgbotapi.ModeMarkdown
		bot.Send(msg)
	} else if callbackData == "ignore" {
		log.Printf("Request to ignore update received. ChatID: %d, MessageID: %d", chatID, messageID)
		answerCallback := tgbotapi.NewCallback(callback.ID, "Запрос проигнорирован.")
		bot.Send(answerCallback)
		msg := tgbotapi.NewMessage(chatID, "Запрос на обновление проигнорирован.")
		bot.Send(msg)
	}
}

func checkUpdates(appConfig *config.AppConfig, bot *tgbotapi.BotAPI) {
	// 1. Клонируем или обновляем репозиторий
	repoPath := appConfig.Git.LocalRepoPath
	log.Printf("Cloning repository %s to %s", appConfig.Git.RepoURL, repoPath)
	err := gitrepo.CloneOrPull(appConfig.Git.RepoURL, appConfig.Git.Branch, repoPath, appConfig.Git.SSHKeyPath)
	if err != nil {
		log.Printf("Failed to clone or pull repository: %v", err)
		return
	}
	log.Println("Repository cloned or pulled successfully.")

	// 2. Находим релевантные файлы
	filePaths, err := checker.FindRelevantFiles(repoPath)
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
				RepoPath: repoPath,
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
