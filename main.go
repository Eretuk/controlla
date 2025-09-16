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
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Masterminds/semver"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// InfoCache хранит информацию о найденных обновлениях
type UpdateInfo struct {
	NewTag   string
	Services []ServiceInfo // Список всех сервисов, которые используют этот образ
}
type ServiceInfo struct {
	FilePath    string
	ServiceName string
	Image       string
	Tag         string
	Registry    string
}

var updateCache = make(map[string]UpdateInfo)
var cacheMutex sync.Mutex // Используем мьютекс для безопасного доступа к кешу

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

	// 1. Создаем контекст для корректного завершения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем канал для сигналов завершения (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем горутину для прослушивания Telegram
	go startTelegramListener(ctx, appConfig.TelegramToken)

	// 2. Основная логика проверки обновлений
	log.Println("Starting image update check...")

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
				// Генерируем уникальный ID для кеша
				id := fmt.Sprintf("%x", time.Now().UnixNano())
				// Сохраняем информацию об обновлении в кеш
				cacheMutex.Lock()
				updateCache[id] = UpdateInfo{
					RepoPath:    "путь_к_твоему_репозиторию",         // Замени на реальный путь
					FilePath:    "путь_к_твоему_docker-compose.yaml", // Замени на реальный путь
					ServiceName: "имя_сервиса",                       // Замени на реальное имя сервиса
					Image:       parsedImage.Image,
				}
				cacheMutex.Unlock()
				log.Printf("🚨 New update for %s. Current tag: %s, Avaliable: %s", parsedImage.Image, parsedImage.Tag, latestAvailable)
				log.Printf("Send notification to telegram")
				message := fmt.Sprintf("🚨 New update for %s\nCurrent version: `%s`\nAvaliable version: `%s`", parsedImage.Image, parsedImage.Tag, latestAvailable)
				notifier.SendInteractiveMessage(appConfig.TelegramToken, appConfig.TelegramChatID, message, latestTags)
			} else {
				log.Printf("✅ %s is actual. Current tag: %s", parsedImage.Image, parsedImage.Tag)
			}
		} else {
			// log.Printf("Skipping image %s: registry %s is not yet supported.", imageString, parsedImage.Registry)
		}
	}
	log.Println("Image check completed. Waiting for termination signal...")

	// 3. Блокируем main горутину, чтобы программа не завершилась
	<-sigChan
	log.Println("Shutting down...")
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

// startTelegramListener запускает слушателя для Telegram API.
func startTelegramListener(ctx context.Context, token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Error creating Telegram bot API client: %v", err)
		return
	}

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
		err := updater.UpdateAndPushService(info.RepoPath, info.FilePath, info.ServiceName, info.NewTag) // NewTag нужно получить из кнопок
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
