package handlers

import (
    "fmt"
 
     "gowa/utils"  // ← IMPORT PACKAGE UTILS
    "gowa/database"

    "go.mau.fi/whatsmeow"

    "go.mau.fi/whatsmeow/types/events"
)

// MessageHandler struct untuk menyimpan client dan fungsi broadcast
type MessageHandler struct {
    Client          *whatsmeow.Client
    BroadcastFunc   func(interface{}) // Fungsi broadcast ke WebSocket
}

// NewMessageHandler membuat instance baru
func NewMessageHandler(client *whatsmeow.Client, broadcastFunc func(interface{})) *MessageHandler {
    return &MessageHandler{
        Client:        client,
        BroadcastFunc: broadcastFunc,
    }
}

// HandleIncomingMessage - fungsi utama untuk menangani pesan masuk
func (h *MessageHandler) HandleIncomingMessage(v *events.Message) {
    // Cegah pesan dari diri sendiri
    if v.Info.IsFromMe {
        return
    }
    
    // Deteksi jenis chat dari JID
    chatType := utils.DetectChatType(v.Info.Chat)
    
    // Proses berdasarkan jenis chat
    switch chatType {
    case "PRIVATE":
        h.handlePrivateChat(v)
    case "GROUP":
        h.handleGroupChat(v)
    case "BROADCAST":
        h.handleBroadcast(v)
    case "NEWSLETTER":
        h.handleNewsletter(v)
    case "STATUS":
        h.handleStatus(v)
    default:
        fmt.Printf("Unknown chat type: %s\n", chatType)
    }
}

// handlePrivateChat - penanganan chat pribadi
func (h *MessageHandler) handlePrivateChat(v *events.Message) {
    content := utils.ExtractMessageContent(v.Message)
    if content == "" {
        return
    }
    
    // Tentukan JID penerima (bot)
    botJID := h.Client.Store.ID.String()
    senderJID := v.Info.Sender.String()
    chatJID := v.Info.Chat.String()
    
    // Jika sender == chat (personal chat), gunakan botJID sebagai penerima
    if senderJID == chatJID {
        chatJID = botJID
    }
    
    // Simpan ke database
    err := database.SaveMessage(senderJID, chatJID, content, false)
    if err != nil {
        fmt.Printf("Failed to save private message: %v\n", err)
        return
    }
    
    fmt.Printf("✅ [PRIVATE] %s: %s\n", senderJID, content)
    
    // Broadcast ke WebSocket
    if h.BroadcastFunc != nil {
        h.BroadcastFunc(map[string]interface{}{
            "type": "new_message",
            "message": map[string]interface{}{
                "from_jid":   senderJID,
                "to_jid":     chatJID,
                "content":    content,
                "is_from_me": false,
                "timestamp":  v.Info.Timestamp,
                "chat_type":  "PRIVATE",
            },
        })
    }
}

// handleGroupChat - penanganan chat grup
func (h *MessageHandler) handleGroupChat(v *events.Message) {
    content := utils.ExtractMessageContent(v.Message)
    if content == "" {
        return
    }
    
    senderJID := v.Info.Sender.String()
    groupJID := v.Info.Chat.String()
    
    // Simpan ke database
    err := database.SaveMessage(senderJID, groupJID, content, false)
    if err != nil {
        fmt.Printf("Failed to save group message: %v\n", err)
        return
    }
    
    fmt.Printf("✅ [GROUP] %s in %s: %s\n", senderJID, groupJID, content)
    
    // Broadcast ke WebSocket
    if h.BroadcastFunc != nil {
        h.BroadcastFunc(map[string]interface{}{
            "type": "new_message",
            "message": map[string]interface{}{
                "from_jid":   senderJID,
                "to_jid":     groupJID,
                "content":    content,
                "is_from_me": false,
                "timestamp":  v.Info.Timestamp,
                "chat_type":  "GROUP",
            },
        })
    }
}

// handleBroadcast - penanganan siaran (biasanya tidak perlu disimpan)
func (h *MessageHandler) handleBroadcast(v *events.Message) {
    fmt.Printf("📢 [BROADCAST] from: %s\n", v.Info.Sender)
    // Skip, tidak perlu disimpan ke database
}

// handleNewsletter - penanganan channel/newsletter
func (h *MessageHandler) handleNewsletter(v *events.Message) {
    fmt.Printf("📰 [NEWSLETTER] update from: %s\n", v.Info.Sender)
    // Channel tidak bisa dibalas, skip simpan
}

// handleStatus - penanganan status
func (h *MessageHandler) handleStatus(v *events.Message) {
    fmt.Printf("💬 [STATUS] from: %s\n", v.Info.Sender)
    // Skip, tidak perlu disimpan
}