package notifier

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendTelegramMessage отправляет сообщение в указанный чат
func SendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message to Telegram: %v", err)
	} else {
		log.Println("Message sent to Telegram successfully.")
	}
}

// SendInteractiveMessage отправляет сообщение с inline-кнопками.
func SendInteractiveMessage(bot *tgbotapi.BotAPI, chatID string, message string, latestTags []string, callbackData string) {
	latestTag := latestTags[0]
	updateButton := tgbotapi.NewInlineKeyboardButtonData(latestTag, callbackData)
	ignoreButton := tgbotapi.NewInlineKeyboardButtonData("Ignore", strings.Replace(callbackData, "update_to:", "ignore:", 1))

	// Размещаем обе кнопки в одном ряду.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(updateButton, ignoreButton),
	)

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования
	msg.ReplyMarkup = keyboard

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending interactive message to Telegram: %v", err)
	} else {
		log.Println("Interactive message sent to Telegram successfully.")
	}
}
