package main

import (
    "context"
    "os"
    "sync"
    "gowa/routes"

    "github.com/gin-gonic/gin"
     _ "modernc.org/sqlite"  // Driver untuk SQLite
    "github.com/mdp/qrterminal/v3"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

var (
    client *whatsmeow.Client
    qrCode string  // Variable untuk menyimpan QR code
    mu     sync.RWMutex  // Mutex untuk keamanan concurrent access
)

func main() {
    ctx := context.Background()
    dbLogger := waLog.Stdout("Database", "DEBUG", true)

    // Gunakan SQLite (database akan otomatis dibuat)
    // Parameter ?_foreign_keys=on WAJIB untuk whatsmeow[citation:2][citation:4]
     dsn := "file:whatsapp.db?_pragma=foreign_keys(1)"

    container, err := sqlstore.New(ctx,"sqlite", dsn, dbLogger)
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
                // Console QR tetap ada untuk debugging
                qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
            }
        }
    }()

    if err := client.Connect(); err != nil {
        panic(err)
    }

    // API server
    r := gin.Default()
    
    // Endpoint QR Code
    r.GET("/api/qr", func(c *gin.Context) {
        mu.RLock()
        code := qrCode
        mu.RUnlock()
        
        if client.IsLoggedIn() {
            c.JSON(200, gin.H{
                "status": "connected",
                "message": "Already logged in",
            })
            return
        }
        
        if code == "" {
            c.JSON(200, gin.H{
                "status": "waiting",
                "message": "Waiting for QR code...",
            })
            return
        }
        
        c.JSON(200, gin.H{
            "status": "pending",
            "qr_code": code,
        })
    })
    
    // Endpoint status lengkap
    r.GET("/api/status-full", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "is_connected": client.IsConnected(),
            "is_logged_in": client.IsLoggedIn(),
            "qr_available": qrCode != "",
        })
    })
    
    // Routes WhatsApp
    routes.WaRoutes(r, client)

    r.LoadHTMLGlob("templates/*")
    r.GET("/", func(c *gin.Context) {
        c.HTML(200, "index.html", gin.H{})
    })

    r.Run(":8080")
}