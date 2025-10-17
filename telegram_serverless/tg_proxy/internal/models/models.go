package models

import "encoding/json"

type TelegramUpdate struct {
	UpdateID    int             `json:"update_id"`
	Message     *Message        `json:"message,omitempty"`
	Callback    *CallbackQuery  `json:"callback_query,omitempty"`
	InlineQuery *InlineQuery    `json:"inline_query,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      *Chat  `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
}

type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type OutgoingCommand struct {
	BotToken string                 `json:"bot_token"`
	Method   string                 `json:"method"`
	Params   map[string]interface{} `json:"params"`
}

type InlineQuery struct {
	ID     string `json:"id"`
	From   *User  `json:"from"`
	Query  string `json:"query"`
	Offset string `json:"offset"`
}
