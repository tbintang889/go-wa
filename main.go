package main

import (
    "context"
    "os"
    "sync"

    "gowa/routes"

    "github.com/gin-gonic/gin"
    _ "modernc.org/sqlite"
    "github.com/mdp/qrterminal/v3"
    "go.mau.fi/whatsmeow"
    waProto "go.mau.fi/whatsmeow/binary/proto"  // ← ALIAS: waProto
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    waLog "go.mau.fi/whatsmeow/util/log"
    "google.golang.org/protobuf/proto"  // ← ini tetap proto
)

var (
    client *whatsmeow.Client
    qrCode string
    mu     sync.RWMutex
)

func main() {
    ctx := context.Background()
    dbLogger := waLog.Stdout("Database", "DEBUG", true)

    // Gunakan SQLite (database akan otomatis dibuat)
    dsn := "file:whatsapp.db?_pragma=foreign_keys(1)"

    container, err := sqlstore.New(ctx, "sqlite", dsn, dbLogger)
    if err != nil {
        panic(err)
    }

    deviceStore, err := container.GetFirstDevice(ctx)
    if err != nil {
        panic(err)
    }

    client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "DEBUG", true))

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
            }
        }
    }()

    if err := client.Connect(); err != nil {
        panic(err)
    }

    // API server
    r := gin.Default()
    
    // ==================== QR CODE ENDPOINTS ====================
    r.GET("/api/qr", func(c *gin.Context) {
        mu.RLock()
        code := qrCode
        mu.RUnlock()
        
        if client.IsLoggedIn() {
            c.JSON(200, gin.H{
                "status":  "connected",
                "message": "Already logged in",
            })
            return
        }
        
        if code == "" {
            c.JSON(200, gin.H{
                "status":  "waiting",
                "message": "Waiting for QR code...",
            })
            return
        }
        
        c.JSON(200, gin.H{
            "status":  "pending",
            "qr_code": code,
        })
    })
    
    r.GET("/api/status-full", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "is_connected": client.IsConnected(),
            "is_logged_in": client.IsLoggedIn(),
            "qr_available": qrCode != "",
        })
    })
    
    // ==================== CHAT ROOM ENDPOINTS ====================
    r.GET("/chat", func(c *gin.Context) {
        c.HTML(200, "chat.html", gin.H{})
    })
    
    r.GET("/api/chats", func(c *gin.Context) {
        contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        
        chatList := []gin.H{}
        for jid, contact := range contacts {
            chatList = append(chatList, gin.H{
                "jid":    jid,
                "name":   contact.PushName,
                "number": jid,
            })
        }
        c.JSON(200, chatList)
    })
    
    r.GET("/api/messages/:jid", func(c *gin.Context) {
        // TODO: Implement dengan database untuk menyimpan riwayat pesan
        c.JSON(200, []gin.H{})
    })
    
    r.POST("/api/send-message", func(c *gin.Context) {
        var req struct {
            To   string `json:"to"`
            Text string `json:"text"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }
        
        jid := types.JID{User: req.To, Server: "s.whatsapp.net"}
        
        // Gunakan waProto (bukan proto) untuk Message
        _, err := client.SendMessage(context.Background(), jid, &waProto.Message{
            Conversation: proto.String(req.Text),  // proto.String dari google.golang.org/protobuf/proto
        })
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        
        c.JSON(200, gin.H{"status": "sent"})
    })
    
    // ==================== WHATSAPP ROUTES ====================
    routes.WaRoutes(r, client)

    // ==================== TEMPLATES ====================
    r.LoadHTMLGlob("templates/*")
    r.GET("/", func(c *gin.Context) {
        c.HTML(200, "index.html", gin.H{})
    })

    r.Run(":8080")
}