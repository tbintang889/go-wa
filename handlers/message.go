package handlers

import (
	"context"
	"fmt"
	"os"
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

// HandleIncomingMessage - fungsi utama untuk menangani pesan masuk
func (h *MessageHandler) HandleIncomingMessage(v *events.Message) {
	// Filter out WA Status, Broadcast list, and Newsletter (Saluran)
	if v.Info.Chat.User == "status" || v.Info.Chat.Server == "broadcast" || v.Info.Chat.Server == "newsletter" || v.Info.Sender.Server == "newsletter" || v.Info.Sender.User == "status" {
		return
	}

	// Deteksi jenis chat
	chatType := utils.DetectChatType(v.Info.Chat)
	if !utils.IsChatable(chatType) {
		return
	}

	// Log pesan masuk untuk debug
	fmt.Printf("📨 [DEBUG] Message received. IsFromMe: %v, Sender: %s, Chat: %s (Type: %s)\n",
		v.Info.IsFromMe, v.Info.Sender, v.Info.Chat, chatType)

	// Ekstrak konten
	content := utils.ExtractMessageContent(v.Message)

	// Proses berdasarkan jenis chat
	switch chatType {
	case "PRIVATE":
		h.handlePrivateChat(v, content)
	case "GROUP":
		h.handleGroupChat(v, content)
	}
}

// handlePrivateChat - penanganan chat pribadi
// handlePrivateChat - penanganan chat pribadi
func (h *MessageHandler) handlePrivateChat(v *events.Message, content string) {
	senderJID := v.Info.Sender.String()
	chatJID := v.Info.Chat.String()

	// Cek apakah ini pesan media
	mediaPath, mediaType := h.processMedia(v, &content)

	// Untuk personal chat, pastikan dari_jid dan to_jid berbeda
	botJID := h.Client.Store.ID.String()
	if senderJID == chatJID {
		chatJID = botJID
	}

	// Simpan ke database
	var err error
	if mediaPath != "" {
		err = database.SaveMessageWithMedia(senderJID, chatJID, content, mediaPath, mediaType, v.Info.IsFromMe)
	} else {
		err = database.SaveMessage(senderJID, chatJID, content, v.Info.IsFromMe)
	}
	if err != nil {
		fmt.Printf("❌ Failed to save private message: %v\n", err)
		return
	}

	fmt.Printf("✅ [PRIVATE] %s -> %s: %s [media: %s]\n", senderJID, chatJID, content, mediaPath)

	// Ambil push name
	pushName := h.getPushName(v.Info.Sender)
	if pushName == "" {
		pushName = extractNumberFromJID(senderJID)
	}

	// Broadcast ke WebSocket - PASTIKAN media_path dan media_type dikirim
	if h.BroadcastFunc != nil {
		msg := map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"from_jid":    senderJID,
				"to_jid":      chatJID,
				"content":     content,
				"is_from_me":  v.Info.IsFromMe,
				"timestamp":   v.Info.Timestamp,
				"chat_type":   "PRIVATE",
				"push_name":   pushName,
				"media_path":  mediaPath,   // ← WAJIB
				"media_type":  mediaType,   // ← WAJIB
			},
		}
		h.BroadcastFunc(msg)
	}
}
// handleGroupChat - penanganan chat grup
func (h *MessageHandler) handleGroupChat(v *events.Message, content string) {
	senderJID := v.Info.Sender.String()
	groupJID := v.Info.Chat.String()

	// Cek apakah ini pesan media
	mediaPath, mediaType := h.processMedia(v, &content)

	// Ambil push name pengirim
	senderPushName := h.getPushName(v.Info.Sender)
	if senderPushName == "" {
		if v.Info.Sender.User != "" {
			senderPushName = v.Info.Sender.User
		} else {
			senderPushName = extractNumberFromJID(senderJID)
		}
	}
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
	if mediaPath != "" {
		err = database.SaveMessageWithMedia(senderJID, groupJID, content, mediaPath, mediaType, v.Info.IsFromMe)
	} else {
		err = database.SaveMessage(senderJID, groupJID, content, v.Info.IsFromMe)
	}
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
				"sender_name":  senderPushName,
				"sender_jid":   senderJID,
				"group_name":   groupName,
				"group_jid":    groupJID,
				"media_path":   mediaPath,
				"media_type":   mediaType,
			},
		})
	}
}

// processMedia - download dan simpan file media, return mediaPath dan mediaType
func (h *MessageHandler) processMedia(v *events.Message, content *string) (string, string) {
	var mediaPath, mediaType string

	switch {
	case v.Message.ImageMessage != nil:
		mediaPath, mediaType = h.downloadMedia(v, "image")
		*content = "📷 Image"
		if caption := v.Message.ImageMessage.GetCaption(); caption != "" {
			*content = "📷 Image: " + caption
		}

	case v.Message.DocumentMessage != nil:
		mediaPath, mediaType = h.downloadMedia(v, "document")
		fileName := v.Message.DocumentMessage.GetFileName()
		if fileName != "" {
			*content = "📄 Document: " + fileName
		} else {
			*content = "📄 Document"
		}

	case v.Message.VideoMessage != nil:
		mediaPath, mediaType = h.downloadMedia(v, "video")
		*content = "📹 Video"

	case v.Message.AudioMessage != nil:
		mediaPath, mediaType = h.downloadMedia(v, "audio")
		*content = "🎵 Voice Note"

	case v.Message.StickerMessage != nil:
		mediaPath, mediaType = h.downloadMedia(v, "sticker")
		*content = "🎨 Sticker"
	}

	return mediaPath, mediaType
}

// downloadMedia - download dan simpan file media
// downloadMedia - download dan simpan file media
func (h *MessageHandler) downloadMedia(v *events.Message, mediaType string) (string, string) {
	var data []byte
	var fileName string
	var err error

	// Buat direktori
	os.MkdirAll("media/images", 0755)
	os.MkdirAll("media/documents", 0755)
	os.MkdirAll("media/videos", 0755)
	os.MkdirAll("media/audios", 0755)
	os.MkdirAll("media/stickers", 0755)

	ctx := context.Background()
	timestamp := time.Now().UnixNano()
	senderNum := extractNumberFromJID(v.Info.Sender.String())

	switch mediaType {
	case "image":
		img := v.Message.ImageMessage
		data, err = h.Client.Download(ctx, img)
		if err != nil {
			fmt.Printf("Failed to download image: %v\n", err)
			return "", ""
		}
		ext := "jpg"
		if mime := img.GetMimetype(); mime != "" {
			if strings.Contains(mime, "png") {
				ext = "png"
			} else if strings.Contains(mime, "gif") {
				ext = "gif"
			} else if strings.Contains(mime, "webp") {
				ext = "webp"
			}
		}
		fileName = fmt.Sprintf("media/images/%d_%s.%s", timestamp, senderNum, ext)

	case "document":
		doc := v.Message.DocumentMessage
		data, err = h.Client.Download(ctx, doc)
		if err != nil {
			fmt.Printf("Failed to download document: %v\n", err)
			return "", ""
		}
		originalName := doc.GetFileName()
		if originalName == "" {
			originalName = "document"
		}
		// Bersihkan nama file
		originalName = strings.ReplaceAll(originalName, "/", "_")
		originalName = strings.ReplaceAll(originalName, "\\", "_")
		originalName = strings.ReplaceAll(originalName, " ", "_")
		fileName = fmt.Sprintf("media/documents/%d_%s_%s", timestamp, senderNum, originalName)

	case "video":
		video := v.Message.VideoMessage
		data, err = h.Client.Download(ctx, video)
		if err != nil {
			fmt.Printf("Failed to download video: %v\n", err)
			return "", ""
		}
		fileName = fmt.Sprintf("media/videos/%d_%s.mp4", timestamp, senderNum)

	case "audio":
		audio := v.Message.AudioMessage
		data, err = h.Client.Download(ctx, audio)
		if err != nil {
			fmt.Printf("Failed to download audio: %v\n", err)
			return "", ""
		}
		fileName = fmt.Sprintf("media/audios/%d_%s.ogg", timestamp, senderNum)

	case "sticker":
		sticker := v.Message.StickerMessage
		data, err = h.Client.Download(ctx, sticker)
		if err != nil {
			fmt.Printf("Failed to download sticker: %v\n", err)
			return "", ""
		}
		fileName = fmt.Sprintf("media/stickers/%d_%s.webp", timestamp, senderNum)

	default:
		return "", ""
	}

	if err != nil || len(data) == 0 {
		return "", ""
	}

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Printf("Failed to save file: %v\n", err)
		return "", ""
	}

	fmt.Printf("📁 Media saved: %s (%s, %d bytes)\n", fileName, mediaType, len(data))
	return fileName, mediaType
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

// HandleHistorySync - menangani sync riwayat pesan & kontak dari WhatsApp
func (h *MessageHandler) HandleHistorySync(v *events.HistorySync) {
	if v == nil || v.Data == nil {
		return
	}

	conversations := v.Data.GetConversations()
	fmt.Printf("🔄 [HISTORY] Syncing %d conversations from WhatsApp...\n", len(conversations))

	botJID := ""
	if h.Client != nil && h.Client.Store != nil && h.Client.Store.ID != nil {
		botJID = h.Client.Store.ID.String()
	}

	savedCount := 0
	for _, conv := range conversations {
		chatJIDStr := conv.GetID()

		if chatJIDStr == "" || !utils.IsValidChatJID(chatJIDStr) {
			continue
		}

		for _, syncMsg := range conv.GetMessages() {
			msg := syncMsg.GetMessage()
			if msg == nil {
				continue
			}

			key := msg.GetKey()
			if key == nil {
				continue
			}

			isFromMe := key.GetFromMe()
			senderJID := chatJIDStr
			if isFromMe {
				senderJID = botJID
			} else if key.GetParticipant() != "" {
				senderJID = key.GetParticipant()
			}

			content := utils.ExtractMessageContent(msg.GetMessage())
			if content == "" {
				continue
			}

			ts := time.Unix(int64(msg.GetMessageTimestamp()), 0)
			if ts.IsZero() || ts.Unix() <= 0 {
				ts = time.Now()
			}

			if err := database.SaveMessageWithTimestamp(senderJID, chatJIDStr, content, isFromMe, ts); err == nil {
				savedCount++
			}
		}
	}

	fmt.Printf("✅ [HISTORY] Successfully synced %d historical messages into database!\n", savedCount)
	if h.BroadcastFunc != nil {
		h.BroadcastFunc(map[string]interface{}{"type": "history_sync_complete"})
	}
}