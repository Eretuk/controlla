package main

import (
	"context"
	"controlla/internal/checker"
	"controlla/internal/config"
	"controlla/internal/gitrepo"
	"controlla/internal/notifier"
	"controlla/internal/updater"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go startTelegramListener(ctx, appConfig.TelegramToken)

	log.Println("Starting image update check...")
	checkUpdates(appConfig)

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

func checkUpdates(appConfig *config.AppConfig) {
	log.Printf("Cloning repository %s to %s", cfg.Git.RepoURL, repoPath)
	err = gitrepo.CloneOrPull(cfg.Git.RepoURL, cfg.Git.Branch, repoPath, cfg.Git.SSHKeyPath)
	if err != nil {
		log.Fatalf("Failed to clone repository: %v", err)
	}
	log.Println("Repository cloned successfully.")

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

	// 5. Проверяем обновления для каждого уникального образа
	for fullImageName, services := range servicesByImage {
		// Берем первый сервис из группы для проверки тега, так как они все одинаковые
		currentTag := services[0].ImageInfo.Tag
		parsedImage := services[0].ImageInfo

		// ... (остальной код проверки, как у тебя было раньше) ...
		// Добавь здесь свою логику:
		// - проверка на docker.io
		// - получение списка тегов
		// - проверка на SemVer
		// - сравнение версий
		// ...

		// Вот пример логики, если обновление найдено
		if isUpdateAvailable { // Эта переменная должна быть результатом твоей логики
			// Генерируем уникальный ID для кеша
			id := fmt.Sprintf("%x", time.Now().UnixNano())

			// Сохраняем информацию об обновлении в кеш
			cacheMutex.Lock()
			updateCache[id] = UpdateInfo{
				RepoPath: repoPath,
				Services: services,        // Сохраняем весь список
				NewTag:   latestAvailable, // Замени на переменную с последним тегом
			}
			cacheMutex.Unlock()

			message := fmt.Sprintf("🚨 Обновление для образа **%s**\nТекущая версия: `%s`\nДоступная версия: `%s`", parsedImage.Image, currentTag, latestAvailable)

			// Отправляем сообщение, передавая ID
			notifier.SendInteractiveMessage(appConfig.TelegramToken, appConfig.TelegramChatID, message, latestTags, id)
		}
	}
}
