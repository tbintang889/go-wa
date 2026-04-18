package controllers

import (
    "context"
    "io"
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "google.golang.org/protobuf/proto"
)

func SendMedia(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            To        string `json:"to" form:"to"`
            MediaType string `json:"media_type" form:"media_type"` // image, document
            FilePath  string `json:"file_path" form:"file_path"`
            Caption   string `json:"caption" form:"caption"`
            FileName  string `json:"file_name" form:"file_name"`
        }
        
        if err := c.ShouldBind(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        // Baca file
        file, err := os.Open(req.FilePath)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "File not found"})
            return
        }
        defer file.Close()

        _, err = io.ReadAll(file)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
            return
        }

        jid := types.JID{User: req.To, Server: "s.whatsapp.net"}
        
        var msg *waProto.Message

        if req.MediaType == "image" {
            msg = &waProto.Message{
                ImageMessage: &waProto.ImageMessage{
                    Caption: proto.String(req.Caption),
                    Mimetype: proto.String("image/jpeg"),
                    URL:      nil, // Untuk file lokal, kita upload dulu atau kirim sebagai data
                },
            }
            // Untuk mengirim file lokal, perlu upload ke WhatsApp servers terlebih dahulu
            // Atau gunakan cara lain: kirim sebagai document dengan mimetype image
            c.JSON(http.StatusNotImplemented, gin.H{"error": "Image upload not implemented yet"})
            return
        } else if req.MediaType == "document" {
            // Untuk mengirim dokumen, perlu diupload ke server WhatsApp
            c.JSON(http.StatusNotImplemented, gin.H{"error": "Document upload not implemented yet"})
            return
        }

        _, err = client.SendMessage(context.Background(), jid, msg)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{"status": "sent", "to": req.To, "type": req.MediaType})
    }
}

func GetDeliveryStatus(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        messageID := c.Query("message_id")
        // Untuk delivery status, perlu implementasi event handler dan database
        c.JSON(http.StatusOK, gin.H{
            "message_id": messageID,
            "status": "pending",
            "note": "Implement with event handler and database",
        })
    }
}