package handlers

import (
	"context"
	"fmt"
	"strings"
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
	// Log semua pesan masuk untuk debug
	fmt.Printf("📨 [DEBUG] Message received. IsFromMe: %v, Sender: %s, Chat: %s\n", 
		v.Info.IsFromMe, v.Info.Sender, v.Info.Chat)
	
	// JANGAN skip pesan dari diri sendiri dulu, biar keliatan
	// if v.Info.IsFromMe {
	// 	fmt.Println("⏭️ Skip: message from self")
	// 	return
	// }
	
	// Deteksi jenis chat
	chatType := utils.DetectChatType(v.Info.Chat)
	fmt.Printf("📨 [DEBUG] ChatType: %s\n", chatType)
	
	// Ekstrak konten
	content := utils.ExtractMessageContent(v.Message)
	fmt.Printf("📨 [DEBUG] Content: '%s'\n", content)
	
	if content == "" {
		fmt.Println("⏭️ Skip: empty content")
		return
	}
	
	// SIMPAN SEMUA PESAN (termasuk dari diri sendiri untuk test)
	senderJID := v.Info.Sender.String()
	chatJID := v.Info.Chat.String()
	
	// Untuk personal chat, pastikan dari_jid dan to_jid berbeda
	botJID := h.Client.Store.ID.String()
	if senderJID == chatJID && chatType == "PRIVATE" {
		chatJID = botJID
	}
	
	// Simpan ke database
	err := database.SaveMessage(senderJID, chatJID, content, v.Info.IsFromMe)
	if err != nil {
		fmt.Printf("❌ Failed to save message: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Message saved: [%s] %s -> %s: %s\n", chatType, senderJID, chatJID, content)
	
	// Ambil push name
	pushName := h.getPushName(v.Info.Sender)
	if pushName == "" {
		pushName = strings.Split(senderJID, "@")[0]
	}
	
	// Broadcast ke WebSocket (hanya jika bukan dari diri sendiri atau tetap broadcast)
	if h.BroadcastFunc != nil {
		msg := map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"from_jid":   senderJID,
				"to_jid":     chatJID,
				"content":    content,
				"is_from_me": v.Info.IsFromMe,
				"timestamp":  v.Info.Timestamp,
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
}

func (h *MessageHandler) getPushName(sender types.JID) string {
	contact, err := h.Client.Store.Contacts.GetContact(context.Background(), sender)
	if err == nil && contact.PushName != "" {
		return contact.PushName
	}
	return ""
}