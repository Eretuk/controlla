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
func SendInteractiveMessage(bot *tgbotapi.BotAPI, chatID string, message string, latestTags []string, id string) {

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования

	// Создаем кнопки
	var buttons []tgbotapi.InlineKeyboardButton
	for _, tag := range latestTags {
		button := tgbotapi.NewInlineKeyboardButtonData(tag, "update_to:"+id+":"+tag)
		buttons = append(buttons, button)
	}

	// Создаём клавиатуру с кнопками
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttons...),
		// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Добавляем ID к кнопке "Ignore"
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ignore this update", "ignore:"+id),
		),
	)

	msg.ReplyMarkup = keyboard

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending interactive message to Telegram: %v", err)
	} else {
		log.Println("Interactive message sent to Telegram successfully.")
	}
}
