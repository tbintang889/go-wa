package utils

import (
    "strings"
    "time"

    "go.mau.fi/whatsmeow/types"
    waProto "go.mau.fi/whatsmeow/binary/proto"
)

// DetectChatType - deteksi jenis chat dari JID
func DetectChatType(jid types.JID) string {
    switch jid.Server {
    case "s.whatsapp.net":
        return "PRIVATE"
    case "lid":
        return "PRIVATE"
    case "g.us":
        return "GROUP"
    case "broadcast":
        if jid.User == "status" {
            return "STATUS"
        }
        return "BROADCAST"
    case "newsletter":
        return "NEWSLETTER"
    default:
        return "UNKNOWN"
    }
}

// IsChatable - apakah chat ini bisa dibalas/disimpan? (hanya PRIVATE dan GROUP)
func IsChatable(chatType string) bool {
    return chatType == "PRIVATE" || chatType == "GROUP"
}

// ExtractMessageContent - ekstrak konten pesan
func ExtractMessageContent(msg *waProto.Message) string {
    if msg == nil {
        return ""
    }
    
    if msg.GetConversation() != "" {
        return msg.GetConversation()
    }
    
    if msg.ExtendedTextMessage != nil {
        return msg.ExtendedTextMessage.GetText()
    }
    
    if msg.ImageMessage != nil {
        caption := msg.ImageMessage.GetCaption()
        if caption != "" {
            return "📷 Image: " + caption
        }
        return "📷 Image"
    }
    
    if msg.StickerMessage != nil {
        return "🎨 Sticker"
    }
    
    if msg.AudioMessage != nil {
        return "🎵 Voice note"
    }
    
    if msg.VideoMessage != nil {
        return "📹 Video"
    }
    
    if msg.DocumentMessage != nil {
        fileName := msg.DocumentMessage.GetFileName()
        if fileName != "" {
            return "📄 Document: " + fileName
        }
        return "📄 Document"
    }
    
    return ""
}

// NormalizeJID - normalisasi format JID
/* func NormalizeJID(jid string) string {
    if strings.Contains(jid, ":") {
        parts := strings.Split(jid, ":")
        if strings.Contains(parts[0], "@") {
            return parts[0]
        }
        return parts[0] + "@s.whatsapp.net"
    }
    return jid
} */

// NormalizeJID - normalisasi format JID ke format terbaru
func NormalizeJID(jid string) string {
    // Ubah @c.us menjadi @s.whatsapp.net
    if strings.HasSuffix(jid, "@c.us") {
        return strings.TrimSuffix(jid, "@c.us") + "@s.whatsapp.net"
    }
    
    // Ubah @lid menjadi @s.whatsapp.net (opsional, hati-hati)
    // if strings.HasSuffix(jid, "@lid") {
    //     return strings.TrimSuffix(jid, "@lid") + "@s.whatsapp.net"
    // }
    
    // Hapus bagian :64 atau :0 dari JID
    if strings.Contains(jid, ":") {
        parts := strings.Split(jid, ":")
        if strings.Contains(parts[0], "@") {
            return parts[0]
        }
        return parts[0] + "@s.whatsapp.net"
    }
    
    return jid
}
// FormatTimestamp - format timestamp
func FormatTimestamp(t time.Time) string {
    now := time.Now()
    if now.Sub(t) < 24*time.Hour {
        return t.Format("15:04")
    }
    return t.Format("02/01 15:04")
}