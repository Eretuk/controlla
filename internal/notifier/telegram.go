package notifier

import (
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
func SendInteractiveMessage(bot *tgbotapi.BotAPI, chatID string, message string, latestTags []string, fullImageName string) {

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования

	// Проверяем, что список тегов не пуст
	if len(latestTags) == 0 {
		log.Println("No latest tags provided to create interactive message.")
		return
	}

	// Берём только самый последний, самый актуальный тег.
	latestTag := latestTags[0]

	// Создаем две кнопки: одну с тегом (для обновления) и одну для игнорирования.
	updateButton := tgbotapi.NewInlineKeyboardButtonData(latestTag, "update_to:"+fullImageName+":"+latestTag)
	ignoreButton := tgbotapi.NewInlineKeyboardButtonData("Ignore", "ignore:"+fullImageName+":"+latestTag)

	// Размещаем обе кнопки в одном ряду.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(updateButton, ignoreButton),
	)

	msg.ReplyMarkup = keyboard

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending interactive message to Telegram: %v", err)
	} else {
		log.Println("Interactive message sent to Telegram successfully.")
	}
}
