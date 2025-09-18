package notifier

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// Send message to a Telegram chat.
func SendTelegramMessage(bot *tgbotapi.BotAPI, chatID int64, message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.DisableWebPagePreview = true

	_, err := bot.Send(msg)
	if err != nil {
		logrus.Errorf("Error sending message to Telegram: %v", err)
	} else {
		logrus.Infof("Message sent to Telegram successfully.")
	}
}

// SendInteractiveMessage sends a message with inline buttons to a Telegram chat.
// The message includes buttons for updating to the latest tag and ignoring the update.
func SendInteractiveMessage(bot *tgbotapi.BotAPI, chatID string, message string, latestTags []string, callbackData string) {
	latestTag := latestTags[0]
	updateButton := tgbotapi.NewInlineKeyboardButtonData(latestTag, callbackData)
	ignoreButton := tgbotapi.NewInlineKeyboardButtonData("Ignore", strings.Replace(callbackData, "update_to:", "ignore:", 1))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(updateButton, ignoreButton),
	)

	msg := tgbotapi.NewMessageToChannel(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = keyboard

	_, err := bot.Send(msg)
	if err != nil {
		logrus.Errorf("Error sending interactive message to Telegram: %v", err)
	} else {
		logrus.Infof("Interactive message sent to Telegram successfully.")
	}
}
