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

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.SetLevel(logrus.InfoLevel)

	appConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading application configuration: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	// 🚨 ИСПРАВЛЕНИЕ: Инициализируем и загружаем список пропусков.
	sl := skiplist.NewSkipList(appConfig.Checker.SkipListFile)
	if err := sl.Load(); err != nil {
		log.Printf("Error loading skip list: %v", err)
	} else {
		log.Println("Skip list loaded successfully.")
	}

	if _, err := os.Stat(appConfig.Git.LocalRepoPath); err == nil {
		log.Println("Removing old repository clone...")
		if err := os.RemoveAll(appConfig.Git.LocalRepoPath); err != nil {
			log.Fatalf("Error removing old repository: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

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

	checkInterval := time.Duration(appConfig.Checker.IntervalMinutes) * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	runCheck(ctx, appConfig, bot, sl)

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return
		case <-ticker.C:
			runCheck(ctx, appConfig, bot, sl)
		}
	}
}

// 🚨 ИСПРАВЛЕНИЕ: Добавляем 'bot' и 'sl' как параметры функции.
func runCheck(ctx context.Context, appConfig *config.AppConfig, bot *tgbotapi.BotAPI, sl *skiplist.SkipList) {
	log.Println("Starting image update check...")

	repoPath := appConfig.Git.LocalRepoPath

	// 🚨 ИСПРАВЛЕНИЕ: Отладочные выводы для проверки пути к ключу
	log.Printf("Attempting to use SSH key path: %s", appConfig.Git.SSHKeyPath)
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

			// 🚨 ИСПРАВЛЕНИЕ: Проверяем, не пропущено ли это обновление.
			if sl.IsSkipped(fullImageName, services[0].ImageInfo.Tag) {
				log.Printf("Skipping update for %s. It's in the skip list.", fullImageName)
				continue
			}

			// 🚨 ИСПРАВЛЕНИЕ: Заново добавляем логику для выбора правильного клиента реестра
			// и получения тегов. Это было утеряно в предыдущем ответе.
			var tags []string
			var err error

			switch services[0].ImageInfo.Registry {
			case "docker.io":
				client := registry.NewDockerHubClient(appConfig.DockerUser, appConfig.DockerPass)
				tags, err = client.GetTags(services[0].ImageInfo.Image)
			case "ghcr.io":
				client := registry.NewGhcrClient(appConfig.GithubToken)
				tags, err = client.GetTags(services[0].ImageInfo.Image)
			// Если нужны другие реестры, можно добавить их здесь
			default:
				log.Printf("Skipping unsupported registry: %s", services[0].ImageInfo.Registry)
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
				if sl.IsSkipped(fullImageName, latestAvailable) {
					log.Printf("⏩ Update for %s (tag %s) is in the skip list. Skipping notification.", fullImageName, latestAvailable)
					continue // Пропускаем отправку уведомления и переходим к следующему сервису
				}
				log.Printf("🚨 New update for %s. Current tag: %s, Available: %s", fullImageName, services[0].ImageInfo.Tag, latestAvailable)
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

				message := fmt.Sprintf("🚨 New update for **%s**\\nCurrent version: `%s`\\nAvaliable version: `%s`", fullImageName, services[0].ImageInfo.Tag, latestAvailable)
				// 🚨 ИСПРАВЛЕНИЕ: Передаем 'bot' и преобразуем chatID в string, как требует notifier.SendInteractiveMessage.
				notifier.SendInteractiveMessage(bot, appConfig.TelegramChatID, message, latestTags, id)
			} else {
				log.Printf("✅ %s is actual. Current tag: %s", fullImageName, services[0].ImageInfo.Tag)
			}
		}
	}
	log.Println("Image check completed.")
}

// 🚨 ИСПРАВЛЕНИЕ: Добавляем 'bot' и 'sl' как параметры функции.
func handleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, appConfig *config.AppConfig, sl *skiplist.SkipList) {
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

		// 🚨 ИСПРАВЛЕНИЕ: Перебираем все сервисы, которые используют этот образ
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
		id := strings.TrimPrefix(callbackData, "ignore:")
		// Получаем информацию из кеша
		cacheMutex.Lock()
		info, ok := updateCache[id]
		cacheMutex.Unlock()
		if ok {
			// 🚨 ИСПРАВЛЕНИЕ: Используем skipList для вызова метода Add
			fullImageName := registry.GetFullImageName(&info.Services[0].ImageInfo)
			err := sl.Add(fullImageName, info.NewTag)
			if err != nil {
				log.Printf("Error adding update to skip list: %v", err)
			}
		} else {
			log.Printf("Error: Update info for ID %s not found in cache. The program may have been restarted.", id)
		}
		// 🚨 ИСПРАВЛЕНИЕ: Логика для кнопки "Skip"
		log.Printf("Received skip request for update ID: %s", strings.TrimPrefix(callbackData, "ignore:"))

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
