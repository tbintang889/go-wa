package main

import (
	"context"
	"fmt"
	"gowa/database"
	"gowa/handlers"
	"gowa/routes"
	"gowa/utils"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

var (
	client *whatsmeow.Client
	qrCode string
	mu     sync.RWMutex

	// WebSocket clients
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex

	// Upgrader WebSocket
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins (for development)
		},
	}
)

// Broadcast message ke semua WebSocket clients
func broadcastMessage(message interface{}) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()

	for client := range wsClients {
		err := client.WriteJSON(message)
		if err != nil {
			client.Close()
			delete(wsClients, client)
		}
	}
}

// Handle WebSocket connection

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		// Handle send_message dari client
		if msg["type"] == "send_message" {
			data := msg["data"].(map[string]interface{})
			to := data["to"].(string)
			text := data["text"].(string)

			// NORMALISASI JID sebelum kirim
			to = utils.NormalizeJID(to) // ← Tambahkan ini

			// Pastikan ada @s.whatsapp.net
			if !strings.Contains(to, "@") {
				to = to + "@s.whatsapp.net"
			}

			// Parse JID
			jid, err := types.ParseJID(to)
			if err != nil {
				println("Invalid JID:", to, err.Error())
				continue
			}

			// Dapatkan JID bot
			botJID := client.Store.ID.String()

			// Simpan ke database dengan status pending
			result, err := database.DB.Exec(`
                INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) 
                VALUES (?, ?, ?, ?, 'pending', ?)
            `, botJID, to, text, true, time.Now())

			if err != nil {
				println("Failed to save message:", err.Error())
				continue
			}

			messageID, _ := result.LastInsertId()

			// Broadcast ke semua client (termasuk pengirim)
			broadcastMessage(map[string]interface{}{
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

			// Kirim ke WhatsApp di background
			go func(msgID int64, jid types.JID, toNum, msgText string) {
				// Context dengan timeout 15 detik
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				_, err := client.SendMessage(ctx, jid, &waProto.Message{
					Conversation: proto.String(msgText),
				})

				if err != nil {
					println("Failed to send to:", toNum, "-", err.Error())
					database.UpdateMessageStatus(int(msgID), "failed")

					broadcastMessage(map[string]interface{}{
						"type": "message_status",
						"data": map[string]interface{}{
							"id":     msgID,
							"status": "failed",
							"error":  err.Error(),
						},
					})
				} else {
					println("Sent to:", toNum)
					database.UpdateMessageStatus(int(msgID), "sent")

					broadcastMessage(map[string]interface{}{
						"type": "message_status",
						"data": map[string]interface{}{
							"id":     msgID,
							"status": "sent",
						},
					})
				}
			}(messageID, jid, to, text)
		}
	}

	wsClientsMu.Lock()
	delete(wsClients, conn)
	wsClientsMu.Unlock()
}
func main() {
	ctx := context.Background()

	// Init database
	if err := database.InitDB("messages.db"); err != nil {
		panic(err)
	}
	println("Messages database initialized")

	dbLogger := waLog.Stdout("Database", "DEBUG", true)
	// dsn := "file:whatsapp.db?_pragma=foreign_keys(1)&_timeout=5000"
	dsn := "file:whatsapp.db?_pragma=foreign_keys(1)&_timeout=5000&_busy_timeout=5000"

	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLogger)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}

	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "DEBUG", true))
	messageHandler := handlers.NewMessageHandler(client, broadcastMessage)
	// ==================== EVENT HANDLER UNTUK PESAN MASUK ====================
	/* 	client.AddEventHandler(func(evt interface{}) {
		fmt.Printf("DEBUG: Event type: %T\n", evt) // CETAK SEMUA EVENT

		switch v := evt.(type) {
		case *events.Message:
			fmt.Printf("DEBUG: Message received from: %s\n", v.Info.Sender)

			// Extract pesan
			var content string
			if v.Message.GetConversation() != "" {
				content = v.Message.GetConversation()
			}

			if content != "" && !v.Info.IsFromMe {
				// SIMPAN KE DATABASE
				err := database.SaveMessage(
					v.Info.Sender.String(),
					v.Info.Chat.String(),
					content,
					false,
				)
				if err != nil {
					fmt.Printf("ERROR save to DB: %v\n", err)
				} else {
					fmt.Printf("SUCCESS: Message saved to DB\n")
				}

				// BROADCAST KE WEBSOCKET
				broadcastMessage(map[string]interface{}{
					"type": "new_message",
					"message": map[string]interface{}{
						"from_jid":   v.Info.Sender.String(),
						"to_jid":     v.Info.Chat.String(),
						"content":    content,
						"is_from_me": false,
						"timestamp":  time.Now(),
					},
				})
			}
		}
	}) */
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {

		case *events.Message:
			messageHandler.HandleIncomingMessage(v)
		case *events.HistorySync:
			// JANGAN simpan history sync ke messages table
			fmt.Printf("History sync received, ignoring: %d conversations\n", len(v.Data.GetConversations()))
			return

		case *events.PushName:
			// JANGAN simpan push name update
			fmt.Printf("PushName update ignored: %s -> %s\n", v.JID, v.NewPushName)
			return

		default:
			// Event lain diabaikan
			// fmt.Printf("Unhandled event: %T\n", evt)
		}
	})
	// Setup QR channel
	qrChan, _ := client.GetQRChannel(ctx)
	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				mu.Lock()
				qrCode = evt.Code
				mu.Unlock()
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "success" {
				println("Login successful!")
				mu.Lock()
				qrCode = ""
				mu.Unlock()

				// Broadcast login success
				broadcastMessage(map[string]interface{}{
					"type":    "login_success",
					"message": "WhatsApp connected",
				})
			}
		}
	}()

	if err := client.Connect(); err != nil {
		panic(err)
	}

	// API server
	r := gin.Default()

	// WebSocket endpoint
	r.GET("/ws", handleWebSocket)

	// QR Code endpoints
	r.GET("/api/qr", func(c *gin.Context) {
		mu.RLock()
		code := qrCode
		mu.RUnlock()

		if client.IsLoggedIn() {
			c.JSON(200, gin.H{"status": "connected", "message": "Already logged in"})
			return
		}

		if code == "" {
			c.JSON(200, gin.H{"status": "waiting", "message": "Waiting for QR code..."})
			return
		}

		c.JSON(200, gin.H{"status": "pending", "qr_code": code})
	})

	r.GET("/api/status-full", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"is_connected": client.IsConnected(),
			"is_logged_in": client.IsLoggedIn(),
			"qr_available": qrCode != "",
		})
	})

	// Chat Room endpoints
	r.GET("/chat", func(c *gin.Context) {
		c.HTML(200, "chat.html", gin.H{})
	})

	// var contactsSynced = false // Tambahkan flag global di awal

	r.GET("/api/chats", func(c *gin.Context) {
    // Ambil chat unik dari database messages
    rows, err := database.DB.Query(`
        SELECT DISTINCT from_jid FROM messages WHERE from_jid != to_jid AND content != ''
        UNION 
        SELECT DISTINCT to_jid FROM messages WHERE from_jid != to_jid AND content != ''
    `)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    chatList := []gin.H{}
    for rows.Next() {
        var jidStr string
        if err := rows.Scan(&jidStr); err != nil {
            continue
        }
        
        // Parse JID untuk ambil nama
        jid, err := types.ParseJID(jidStr)
        name := jidStr
        if err == nil {
            contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
            if err == nil && contact.PushName != "" {
                name = contact.PushName
            }
        }
        
        chatList = append(chatList, gin.H{
            "jid":    jidStr,
            "name":   name,
            "number": jidStr,
        })
    }
    
    c.JSON(200, chatList)
})
	// Di main.go, pastikan ada endpoint ini
	r.GET("/api/messages/:jid", func(c *gin.Context) {
		jid := c.Param("jid")

		// Decode URL encoding (%40 = @)
		decodedJID, err := url.QueryUnescape(jid)
		if err != nil {
			decodedJID = jid
		}

		page := 1
		if pageStr := c.Query("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		limit := 10
		offset := (page - 1) * limit

		// Query ke database messages.db
		query := `SELECT id, from_jid, to_jid, content, is_from_me, timestamp 
              FROM messages 
              WHERE from_jid = ? OR to_jid = ?
              ORDER BY timestamp DESC 
              LIMIT ? OFFSET ?`

		rows, err := database.DB.Query(query, decodedJID, decodedJID, limit, offset)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var messages []database.Message
		for rows.Next() {
			var msg database.Message
			err := rows.Scan(&msg.ID, &msg.FromJID, &msg.ToJID, &msg.Content, &msg.IsFromMe, &msg.Timestamp)
			if err != nil {
				continue
			}
			messages = append(messages, msg)
		}

		// Balik urutan
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		// Hitung total
		var total int
		countQuery := `SELECT COUNT(*) FROM messages WHERE from_jid = ? OR to_jid = ?`
		database.DB.QueryRow(countQuery, decodedJID, decodedJID).Scan(&total)

		c.JSON(200, gin.H{
			"messages":    messages,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"has_more":    total > (page * limit),
			"total_pages": (total + limit - 1) / limit,
		})
	})

	// GET /api/messages/:jid - Ambil 10 pesan terbaru
	// GET /api/messages/:jid?page=2 - Ambil 10 pesan berikutnya
	r.POST("/api/send-message", func(c *gin.Context) {
		var req struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Normalisasi JID
		to := utils.NormalizeJID(req.To)
		if !strings.Contains(to, "@") {
			to = to + "@s.whatsapp.net"
		}

		_, err := types.ParseJID(to)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid JID"})
			return
		}

		// ... sisanya sama
	})
	routes.WaRoutes(r, client)
	r.LoadHTMLGlob("templates/*")
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{})
	})
	// Endpoint untuk mengakses file media
	r.GET("/api/media/:id", func(c *gin.Context) {
		id := c.Param("id")

		var mediaPath string
		err := database.DB.QueryRow("SELECT media_path FROM messages WHERE id = ?", id).Scan(&mediaPath)
		if err != nil || mediaPath == "" {
			c.String(404, "Media not found")
			return
		}

		// Cek apakah file exists
		if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
			c.String(404, "Media file not found")
			return
		}

		c.File(mediaPath)
	})
	// Auto open browser
	go func() {
		println("Server running at http://localhost:8080")
		time.Sleep(2 * time.Second)
		openBrowser("http://localhost:8080")
		retryPendingMessages()
	}()

	r.Run(":8080")
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		println("Failed to open browser:", err.Error())
	}
}

// Background worker untuk retry pesan yang gagal
func retryPendingMessages() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		messages, err := database.GetPendingMessages()
		if err != nil {
			continue
		}

		for _, msg := range messages {
			println("Retrying message:", msg.ID)

			jid := types.JID{User: msg.ToJID, Server: "s.whatsapp.net"}
			_, err := client.SendMessage(context.Background(), jid, &waProto.Message{
				Conversation: proto.String(msg.Content),
			})

			if err == nil {
				database.UpdateMessageStatus(msg.ID, "sent")
				println("Retry success:", msg.ID)
			}
		}
	}
}

// Di main.go, jalankan worker
