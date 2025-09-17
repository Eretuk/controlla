package notifier

import (
	"fmt"
	"log"

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
func SendInteractiveMessage(bot *tgbotapi.BotAPI, chatID string, message string, latestTags []string, id string) {

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования

	if len(latestTags) > 0 {
		// Кнопка для обновления (только на самый последний тег)
		updateButton := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Update to %s", latestTags[0]), "update_to:"+id+":"+latestTags[0])

		// Кнопка для пропуска
		skipButton := tgbotapi.NewInlineKeyboardButtonData("Skip this update", "skip_update:"+id)

		// Создаем клавиатуру с кнопками
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(updateButton),
			tgbotapi.NewInlineKeyboardRow(skipButton),
		)
		msg.ReplyMarkup = keyboard
	}

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending interactive message to Telegram: %v", err)
	} else {
		log.Println("Interactive message sent to Telegram successfully.")
	}
}
