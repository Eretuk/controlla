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
