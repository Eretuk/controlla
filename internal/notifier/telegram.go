package notifier

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendTelegramMessage отправляет сообщение в указанный чат
func SendTelegramMessage(token, chatID string, message string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Error creating Telegram bot: %v", err)
		return
	}

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	// Используем NewMessageToChannel вместо NewMessage для отправки в канал/чат по ID

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message to Telegram: %v", err)
	} else {
		log.Println("Message sent to Telegram successfully.")
	}
}

// SendInteractiveMessage отправляет сообщение с inline-кнопками.
func SendInteractiveMessage(token, chatID string, message string, latestTags []string, id string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Error creating Telegram bot: %v", err)
		return
	}

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования

	var buttons []tgbotapi.InlineKeyboardButton
	for _, tag := range latestTags {
		button := tgbotapi.NewInlineKeyboardButtonData(tag, "update_to:"+id+":"+tag)
		buttons = append(buttons, button)
	}

	// Создаём клавиатуру с кнопками
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(buttons...),
		// Можно добавить другие кнопки, например, "игнорировать"
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ignore this update", "ignore"),
		),
	)

	msg.ReplyMarkup = keyboard

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending interactive message to Telegram: %v", err)
	} else {
		log.Println("Interactive message sent to Telegram successfully.")
	}
}
