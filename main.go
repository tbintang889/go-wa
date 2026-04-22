package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

	"gowa/database"
	"gowa/handlers"
	"gowa/routes"
	"gowa/utils"
	"net/http"
	"strings"
)

var (
	client      *whatsmeow.Client
	qrCode      string
	mu          sync.RWMutex
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.RWMutex
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

func broadcastMessage(message interface{}) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	for client := range wsClients {
		if err := client.WriteJSON(message); err != nil {
			client.Close()
			delete(wsClients, client)
		}
	}
}

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
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg["type"] == "send_message" {
			data := msg["data"].(map[string]interface{})
			to := data["to"].(string)
			text := data["text"].(string)
			to = utils.NormalizeJID(to)
			if !strings.Contains(to, "@") {
				to = to + "@s.whatsapp.net"
			}
			jid, err := types.ParseJID(to)
			if err != nil {
				continue
			}
			botJID := client.Store.ID.String()
			result, _ := database.DB.Exec(`INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) VALUES (?, ?, ?, ?, 'pending', ?)`,
				botJID, to, text, true, time.Now())
			messageID, _ := result.LastInsertId()
			broadcastMessage(map[string]interface{}{
				"type": "new_message",
				"message": map[string]interface{}{
					"id": messageID, "from_jid": botJID, "to_jid": to,
					"content": text, "is_from_me": true, "status": "pending", "timestamp": time.Now(),
				},
			})
			go func(mid int64, jid types.JID, toNum, txt string) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if _, err := client.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(txt)}); err != nil {
					database.UpdateMessageStatus(int(mid), "failed")
				} else {
					database.UpdateMessageStatus(int(mid), "sent")
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
	if err := database.InitDB("messages.db"); err != nil {
		panic(err)
	}
	fmt.Println("Messages database initialized")

	dbLogger := waLog.Stdout("Database", "DEBUG", true)
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
	client.AddEventHandler(func(evt interface{}) {
	fmt.Printf("🔥 RAW EVENT TYPE: %T\n", evt) // HARUSNYA muncul *events.Message
	
	switch v := evt.(type) {
	case *events.Message:
		fmt.Printf("🔥 MESSAGE FROM: %s\n", v.Info.Sender)
		messageHandler.HandleIncomingMessage(v)
	case *events.HistorySync:
		fmt.Printf("History sync ignored: %d conversations\n", len(v.Data.GetConversations()))
	case *events.PushName:
		fmt.Printf("PushName update: %s -> %s\n", v.JID, v.NewPushName)
	default:
		fmt.Printf("Unhandled event: %T\n", evt)
	}
})

	qrChan, _ := client.GetQRChannel(ctx)
	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				mu.Lock()
				qrCode = evt.Code
				mu.Unlock()
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "success" {
				fmt.Println("Login successful!")
				mu.Lock()
				qrCode = ""
				mu.Unlock()
				broadcastMessage(map[string]interface{}{"type": "login_success", "message": "WhatsApp connected"})
			}
		}
	}()

	if err := client.Connect(); err != nil {
		panic(err)
	}

	r := gin.Default()
	r.GET("/ws", handleWebSocket)
	routes.SetupRoutes(r, client, &qrCode)
	r.LoadHTMLGlob("templates/*")

	go func() {
		fmt.Println("Server running at http://localhost:8084")
		time.Sleep(2 * time.Second)
		openBrowser("http://localhost:8084")
	}()

	r.Run(":8084")
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
		fmt.Println("Failed to open browser:", err.Error())
	}
}