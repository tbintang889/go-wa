package handlers

import (
    "fmt"

    "gowa/database"
    "gowa/utils"

    "go.mau.fi/whatsmeow"

    "go.mau.fi/whatsmeow/types/events"
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
    // Cegah pesan dari diri sendiri
    if v.Info.IsFromMe {
        return
    }
    
    // Deteksi jenis chat dari JID
    chatType := utils.DetectChatType(v.Info.Chat)
    
    // ========== FILTER: HANYA chat pribadi dan grup ==========
    // NON-CHAT (status, broadcast, newsletter) TIDAK diproses
    switch chatType {
    case "PRIVATE":
        h.handlePrivateChat(v)
    case "GROUP":
        h.handleGroupChat(v)
    // case "BROADCAST", "NEWSLETTER", "STATUS":
    //     // TIDAK dilakukan apa-apa (tidak simpan, tidak broadcast)
    //     fmt.Printf("⏭️ Skipping non-chat: %s\n", chatType)
    //     return
    default:
        // Unknown atau non-chat, diabaikan
        fmt.Printf("⏭️ Skipping: %s\n", chatType)
        return
    }
}

// handlePrivateChat - penanganan chat pribadi
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