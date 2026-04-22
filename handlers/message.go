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

	// Proses berdasarkan jenis chat
	switch chatType {
	case "PRIVATE":
		h.handlePrivateChat(v, content)
	case "GROUP":
		h.handleGroupChat(v, content)
	default:
		fmt.Printf("⏭️ Skipping unknown chat type: %s\n", chatType)
	}
}

// handlePrivateChat - penanganan chat pribadi
func (h *MessageHandler) handlePrivateChat(v *events.Message, content string) {
	senderJID := v.Info.Sender.String()
	chatJID := v.Info.Chat.String()

	// Untuk personal chat, pastikan dari_jid dan to_jid berbeda
	botJID := h.Client.Store.ID.String()
	if senderJID == chatJID {
		chatJID = botJID
	}

	// Simpan ke database
	err := database.SaveMessage(senderJID, chatJID, content, v.Info.IsFromMe)
	if err != nil {
		fmt.Printf("❌ Failed to save private message: %v\n", err)
		return
	}

	fmt.Printf("✅ [PRIVATE] %s -> %s: %s\n", senderJID, chatJID, content)

	// Ambil push name
	pushName := h.getPushName(v.Info.Sender)
	if pushName == "" {
		pushName = extractNumberFromJID(senderJID)
	}

	// Broadcast ke WebSocket
	if h.BroadcastFunc != nil {
		h.BroadcastFunc(map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"from_jid":   senderJID,
				"to_jid":     chatJID,
				"content":    content,
				"is_from_me": v.Info.IsFromMe,
				"timestamp":  v.Info.Timestamp,
				"chat_type":  "PRIVATE",
				"push_name":  pushName,
			},
		})
	}
}

// handleGroupChat - penanganan chat grup (dengan nama pengirim)
// handleGroupChat - penanganan chat grup
// handleGroupChat - penanganan chat grup
func (h *MessageHandler) handleGroupChat(v *events.Message, content string) {
	senderJID := v.Info.Sender.String()
	groupJID := v.Info.Chat.String()

	// Ambil push name pengirim
	senderPushName := h.getPushName(v.Info.Sender)
	if senderPushName == "" {
		// Fallback: coba dari v.Info.Sender.User
		if v.Info.Sender.User != "" {
			senderPushName = v.Info.Sender.User
		} else {
			senderPushName = extractNumberFromJID(senderJID)
		}
	}
	
	// Jika masih kosong, gunakan "Unknown"
	if senderPushName == "" {
		senderPushName = "Unknown"
	}
	
	fmt.Printf("📨 [GROUP] Sender: %s, PushName: '%s'\n", senderJID, senderPushName)

	// Ambil nama grup
	groupName := groupJID
	groupInfo, err := h.Client.GetGroupInfo(context.Background(), v.Info.Chat)
	if err == nil && groupInfo.Name != "" {
		groupName = groupInfo.Name
	}

	// Simpan ke database
	err = database.SaveMessage(senderJID, groupJID, content, v.Info.IsFromMe)
	if err != nil {
		fmt.Printf("❌ Failed to save group message: %v\n", err)
		return
	}

	fmt.Printf("✅ [GROUP] %s in %s: %s\n", senderPushName, groupName, content)

	// Broadcast ke WebSocket dengan info pengirim
	if h.BroadcastFunc != nil {
		h.BroadcastFunc(map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"from_jid":     senderJID,
				"to_jid":       groupJID,
				"content":      content,
				"is_from_me":   v.Info.IsFromMe,
				"timestamp":    v.Info.Timestamp,
				"chat_type":    "GROUP",
				"push_name":    senderPushName,
				"sender_name":  senderPushName,  // ← PENTING
				"sender_jid":   senderJID,
				"group_name":   groupName,
				"group_jid":    groupJID,
			},
		})
	}
}

// getPushName - ambil push name dari kontak
func (h *MessageHandler) getPushName(sender types.JID) string {
	contact, err := h.Client.Store.Contacts.GetContact(context.Background(), sender)
	if err == nil && contact.PushName != "" {
		return contact.PushName
	}
	return ""
}

// extractNumberFromJID - ekstrak nomor dari JID
func extractNumberFromJID(jid string) string {
	// Hapus @s.whatsapp.net, @lid, dll
	if idx := strings.Index(jid, "@"); idx != -1 {
		jid = jid[:idx]
	}
	// Hapus :64 atau :0
	if idx := strings.Index(jid, ":"); idx != -1 {
		jid = jid[:idx]
	}
	return jid
}