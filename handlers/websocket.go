package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
"os"
	"net/http"

	"strings"
	"sync"
	"time"

	"gowa/database"
	"gowa/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex
	waClient    *whatsmeow.Client
)

func SetWhatsAppClient(client *whatsmeow.Client) {
	waClient = client
}

func BroadcastMessage(message interface{}) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	for client := range wsClients {
		if err := client.WriteJSON(message); err != nil {
			client.Close()
			delete(wsClients, client)
		}
	}
}

func HandleWebSocket(c *gin.Context) {
	fmt.Println("🔌 [WS] New client connecting...")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("❌ [WS] Upgrade failed: %v\n", err)
		return
	}
	defer conn.Close()

	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()

	fmt.Println("✅ [WS] Client connected successfully")

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			fmt.Printf("❌ [WS] Read error: %v\n", err)
			break
		}

		// 🔥 FORCE PRINT - pasti muncul
		fmt.Printf("🔥🔥🔥 [WS] RAW MESSAGE: type=%v, keys=%v\n", msg["type"], getKeys(msg))

		msgType, ok := msg["type"].(string)
		if !ok {
			fmt.Println("❌ [WS] No type field")
			continue
		}

		fmt.Printf("📨 [WS] Processing type: %s\n", msgType)

		switch msgType {
		case "send_message":
			fmt.Println("📝 [WS] -> send_message")
			go handleSendMessage(conn, msg)
		case "send_media":
			fmt.Println("🖼️ [WS] -> send_media")
			go HandleSendMedia(conn, msg)
		default:
			fmt.Printf("⚠️ [WS] Unknown type: %s\n", msgType)
		}
	}

	wsClientsMu.Lock()
	delete(wsClients, conn)
	wsClientsMu.Unlock()
	fmt.Println("🔌 [WS] Client disconnected")
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// handleSendMedia - dengan lebih banyak log
func HandleSendMedia(conn *websocket.Conn, msg map[string]interface{}) {
	fmt.Println("🔵🔵🔵 [MEDIA] HANDLER START 🔵🔵🔵")

	// Ambil data dari message
	data, ok := msg["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ [MEDIA] No data field or invalid format")
		sendError(conn, "Invalid media data")
		return
	}

	to, _ := data["to"].(string)
	fileDataB64, _ := data["file_data"].(string)
	fileName, _ := data["file_name"].(string)
	fileType, _ := data["file_type"].(string)
	caption, _ := data["caption"].(string)

	fmt.Printf("📤 [MEDIA] To: %s\n", to)
	fmt.Printf("📤 [MEDIA] File: %s (%s)\n", fileName, fileType)
	fmt.Printf("📤 [MEDIA] Caption: %s\n", caption)
	fmt.Printf("📤 [MEDIA] Base64 length: %d\n", len(fileDataB64))

	if waClient == nil {
		fmt.Println("❌ [MEDIA] waClient is nil! Call SetWhatsAppClient first")
		sendError(conn, "WhatsApp client not ready")
		return
	}

	// Decode base64
	fileData, err := base64.StdEncoding.DecodeString(fileDataB64)
	if err != nil {
		fmt.Printf("❌ [MEDIA] Base64 decode failed: %v\n", err)
		sendError(conn, "Failed to decode file")
		return
	}
	fmt.Printf("📦 [MEDIA] Decoded file size: %d bytes (%.2f KB)\n", len(fileData), float64(len(fileData))/1024)

	// Validasi ukuran
	if len(fileData) > 16*1024*1024 {
		fmt.Printf("❌ [MEDIA] File too large: %d bytes\n", len(fileData))
		sendError(conn, "File too large (max 16MB)")
		return
	}

	// Normalisasi JID
	to = utils.NormalizeJID(to)
	if !strings.Contains(to, "@") {
		to = to + "@s.whatsapp.net"
	}
		

	fmt.Printf("📱 [MEDIA] Normalized JID: %s\n", to)

	jid, err := types.ParseJID(to)
	if err != nil {
		fmt.Printf("❌ [MEDIA] Invalid JID: %v\n", err)
		sendError(conn, "Invalid JID")
		return
	}

	// Simpan ke database
	botJID := waClient.Store.ID.String()
	content := fmt.Sprintf("📷 %s", fileName)
	if strings.HasPrefix(fileType, "image/") {
		content = fmt.Sprintf("📷 Image: %s", fileName)
		if caption != "" {
			content = fmt.Sprintf("📷 Image: %s - %s", fileName, caption)
		}
	} else {
		content = fmt.Sprintf("📄 File: %s", fileName)
		if caption != "" {
			content = fmt.Sprintf("📄 File: %s - %s", fileName, caption)
		}
	}

	result, err := database.DB.Exec(`
		INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) 
		VALUES (?, ?, ?, ?, 'pending', ?)
	`, botJID, to, content, true, time.Now())

	var messageID int64
	if err == nil {
		messageID, _ = result.LastInsertId()
		fmt.Printf("💾 [MEDIA] Saved to DB, ID: %d\n", messageID)

		// Broadcast pending message
		BroadcastMessage(map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"id":         messageID,
				"from_jid":   botJID,
				"to_jid":     to,
				"content":    content,
				"is_from_me": true,
				"status":     "pending",
				"timestamp":  time.Now(),
			},
		})
	} else {
		fmt.Printf("❌ [MEDIA] Failed to save to DB: %v\n", err)
	}

	// Upload ke WhatsApp
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var msgToSend *waProto.Message
	var localPath string

	if strings.HasPrefix(fileType, "image/") {
		fmt.Println("🖼️ [MEDIA] Uploading image...")
		uploaded, err := waClient.Upload(ctx, fileData, whatsmeow.MediaImage)
		if err != nil {
			fmt.Printf("❌ [MEDIA] Upload failed: %v\n", err)
			sendError(conn, "Failed to upload: "+err.Error())
			if messageID > 0 {
				database.UpdateMessageStatus(int(messageID), "failed")
			}
			return
		}
		fmt.Printf("✅ [MEDIA] Image uploaded, URL: %s\n", truncateString(uploaded.URL, 60))

		// Simpan file lokal
		os.MkdirAll("media/images", 0755)
		localPath = fmt.Sprintf("media/images/sent_%d_%s", messageID, fileName)
		if err := os.WriteFile(localPath, fileData, 0644); err != nil {
			fmt.Printf("⚠️ Failed to save local image: %v\n", err)
		} else {
			fmt.Printf("✅ Local image saved: %s\n", localPath)
		}

		msgToSend = &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Caption:    proto.String(caption),
				Mimetype:   proto.String(fileType),
				URL:        proto.String(uploaded.URL),
				DirectPath: proto.String(uploaded.DirectPath),
				MediaKey:   uploaded.MediaKey,
				FileLength: proto.Uint64(uint64(len(fileData))),
			},
		}
	} else {
		fmt.Println("📄 [MEDIA] Uploading document...")
		uploaded, err := waClient.Upload(ctx, fileData, whatsmeow.MediaDocument)
		if err != nil {
			fmt.Printf("❌ [MEDIA] Upload failed: %v\n", err)
			sendError(conn, "Failed to upload: "+err.Error())
			if messageID > 0 {
				database.UpdateMessageStatus(int(messageID), "failed")
			}
			return
		}
		fmt.Printf("✅ [MEDIA] Document uploaded\n")

		// Simpan file lokal
		os.MkdirAll("media/documents", 0755)
		localPath = fmt.Sprintf("media/documents/sent_%d_%s", messageID, fileName)
		if err := os.WriteFile(localPath, fileData, 0644); err != nil {
			fmt.Printf("⚠️ Failed to save local document: %v\n", err)
		} else {
			fmt.Printf("✅ Local document saved: %s\n", localPath)
		}

		msgToSend = &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				Title:      proto.String(fileName),
				FileName:   proto.String(fileName),
				Mimetype:   proto.String(fileType),
				URL:        proto.String(uploaded.URL),
				DirectPath: proto.String(uploaded.DirectPath),
				MediaKey:   uploaded.MediaKey,
				FileLength: proto.Uint64(uint64(len(fileData))),
			},
		}
	}

	// Kirim pesan
	fmt.Println("📨 [MEDIA] Sending message to WhatsApp...")
	_, err = waClient.SendMessage(ctx, jid, msgToSend)
	if err != nil {
		fmt.Printf("❌ [MEDIA] Send failed: %v\n", err)
		sendError(conn, "Failed to send: "+err.Error())
		if messageID > 0 {
			database.UpdateMessageStatus(int(messageID), "failed")
		}
		return
	}

	fmt.Println("✅ [MEDIA] Message sent successfully!")

	// Update status dan media_path di database
	if messageID > 0 {
		database.UpdateMessageStatus(int(messageID), "sent")
		
		// Update media_path di database
		if localPath != "" {
			_, err = database.DB.Exec(`UPDATE messages SET media_path = ?, media_type = ? WHERE id = ?`, 
				localPath, "image", messageID)
			if err != nil {
				fmt.Printf("⚠️ Failed to update media_path: %v\n", err)
			} else {
				fmt.Printf("✅ Media path updated in DB: %s\n", localPath)
			}
		}
		
		// Broadcast ulang dengan media_path
		BroadcastMessage(map[string]interface{}{
			"type": "new_message",
			"message": map[string]interface{}{
				"id":         messageID,
				"from_jid":   botJID,
				"to_jid":     to,
				"content":    content,
				"is_from_me": true,
				"status":     "sent",
				"timestamp":  time.Now(),
				"media_path": localPath,
				"media_type": "image",
			},
		})
	}

	// Response ke client
	conn.WriteJSON(map[string]interface{}{
		"type": "media_sent",
		"data": map[string]interface{}{
			"to":   to,
			"file": fileName,
		},
	})

	fmt.Println("🔵🔵🔵 [MEDIA] HANDLER END 🔵🔵🔵")
}
func sendError(conn *websocket.Conn, errMsg string) {
	conn.WriteJSON(map[string]interface{}{
		"type":  "error",
		"error": errMsg,
	})
}
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
func handleSendMessage(conn *websocket.Conn, msg map[string]interface{}) {
	data, ok := msg["data"].(map[string]interface{})
	if !ok {
		return
	}

	to, _ := data["to"].(string)
	text, _ := data["text"].(string)

	if waClient == nil {
		return
	}

	to = utils.NormalizeJID(to)
	if !strings.Contains(to, "@") {
		to = to + "@s.whatsapp.net"
	}

	jid, err := types.ParseJID(to)
	if err != nil {
		fmt.Printf("Invalid JID: %v\n", err)
		return
	}

	botJID := waClient.Store.ID.String()

	// Simpan ke database
	result, err := database.DB.Exec(`
		INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) 
		VALUES (?, ?, ?, ?, 'pending', ?)
	`, botJID, to, text, true, time.Now())

	if err != nil {
		fmt.Printf("Failed to save message: %v\n", err)
		return
	}

	messageID, _ := result.LastInsertId()

	// Broadcast ke UI
	BroadcastMessage(map[string]interface{}{
		"type": "new_message",
		"message": map[string]interface{}{
			"id":         messageID,
			"from_jid":   botJID,
			"to_jid":     to,
			"content":    text,
			"is_from_me": true,
			"status":     "pending",
			"timestamp":  time.Now(),
		},
	})

	// Kirim di background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := waClient.SendMessage(ctx, jid, &waProto.Message{
			Conversation: proto.String(text),
		})

		if err != nil {
			fmt.Printf("Failed to send: %v\n", err)
			database.UpdateMessageStatus(int(messageID), "failed")
			BroadcastMessage(map[string]interface{}{
				"type": "message_status",
				"data": map[string]interface{}{
					"id":     messageID,
					"status": "failed",
					"error":  err.Error(),
				},
			})
		} else {
			database.UpdateMessageStatus(int(messageID), "sent")
			BroadcastMessage(map[string]interface{}{
				"type": "message_status",
				"data": map[string]interface{}{
					"id":     messageID,
					"status": "sent",
				},
			})
		}
	}()
}
