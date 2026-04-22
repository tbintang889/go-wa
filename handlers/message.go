package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"gowa/database"
	"gowa/utils"
)

type MessageHandler struct {
	Client        *whatsmeow.Client
	BroadcastFunc func(interface{})
}

func NewMessageHandler(client *whatsmeow.Client, broadcastFunc func(interface{})) *MessageHandler {
	return &MessageHandler{
		Client:        client,
		BroadcastFunc: broadcastFunc,
	}
}

func (h *MessageHandler) HandleIncomingMessage(v *events.Message) {
	if v.Info.IsFromMe {
		return
	}
	chatType := utils.DetectChatType(v.Info.Chat)
	switch chatType {
	case "PRIVATE":
		h.handlePrivateChat(v)
	case "GROUP":
		h.handleGroupChat(v)
	default:
		fmt.Printf("⏭️ Skipping: %s\n", chatType)
	}
}

func (h *MessageHandler) handlePrivateChat(v *events.Message) {
	content := utils.ExtractMessageContent(v.Message)
	if content == "" {
		return
	}
	botJID := h.Client.Store.ID.String()
	senderJID := v.Info.Sender.String()
	chatJID := v.Info.Chat.String()
	if senderJID == chatJID {
		chatJID = botJID
	}
	err := database.SaveMessage(senderJID, chatJID, content, false)
	if err != nil {
		fmt.Printf("Failed to save private message: %v\n", err)
		return
	}
	fmt.Printf("✅ [PRIVATE] %s: %s\n", senderJID, content)
	pushName := h.getPushName(v.Info.Sender)
	if pushName == "" {
		pushName = strings.Split(senderJID, "@")[0]
	}
	h.broadcastMessage(senderJID, chatJID, content, false, v.Info.Timestamp, "PRIVATE", pushName, "")
}

func (h *MessageHandler) handleGroupChat(v *events.Message) {
	content := utils.ExtractMessageContent(v.Message)
	if content == "" {
		return
	}
	senderJID := v.Info.Sender.String()
	groupJID := v.Info.Chat.String()
	err := database.SaveMessage(senderJID, groupJID, content, false)
	if err != nil {
		fmt.Printf("Failed to save group message: %v\n", err)
		return
	}
	fmt.Printf("✅ [GROUP] %s in %s: %s\n", senderJID, groupJID, content)
	pushName := h.getPushName(v.Info.Sender)
	if pushName == "" {
		pushName = strings.Split(senderJID, "@")[0]
	}
	h.broadcastMessage(senderJID, groupJID, content, false, v.Info.Timestamp, "GROUP", pushName, senderJID)
}

func (h *MessageHandler) getPushName(sender types.JID) string {
	contact, err := h.Client.Store.Contacts.GetContact(context.Background(), sender)
	if err == nil && contact.PushName != "" {
		return contact.PushName
	}
	return ""
}

func (h *MessageHandler) broadcastMessage(fromJID, toJID, content string, isFromMe bool, timestamp time.Time, chatType, pushName, senderJID string) {
	if h.BroadcastFunc == nil {
		return
	}
	msg := map[string]interface{}{
		"type": "new_message",
		"message": map[string]interface{}{
			"from_jid":   fromJID,
			"to_jid":     toJID,
			"content":    content,
			"is_from_me": isFromMe,
			"timestamp":  timestamp,
			"chat_type":  chatType,
			"push_name":  pushName,
		},
	}
	if chatType == "GROUP" {
		msg["message"].(map[string]interface{})["sender_jid"] = senderJID
		msg["message"].(map[string]interface{})["sender_name"] = pushName
	}
	h.BroadcastFunc(msg)
}