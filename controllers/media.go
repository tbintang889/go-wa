package controllers

import (
    "context"
   
    "io"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "google.golang.org/protobuf/proto"
)

// SendMedia - kirim file/gambar via WhatsApp
func SendMedia(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Parse form data
        to := c.PostForm("to")
        caption := c.PostForm("caption")
        
        file, header, err := c.Request.FormFile("file")
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
            return
        }
        defer file.Close()
        
        if to == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Recipient is required"})
            return
        }
        
        // Baca file
        fileData, err := io.ReadAll(file)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
            return
        }
        
        // Dapatkan MIME type
        fileName := header.Filename
        mimeType := header.Header.Get("Content-Type")
        
        // Upload ke WhatsApp
        ctx := context.Background()
        jid := types.JID{User: to, Server: "s.whatsapp.net"}
        
        var msg *waProto.Message
        
        // Cek apakah gambar
        if strings.HasPrefix(mimeType, "image/") {
            // Upload image
            uploaded, err := client.Upload(ctx, fileData, whatsmeow.MediaImage)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image: " + err.Error()})
                return
            }
            
            msg = &waProto.Message{
                ImageMessage: &waProto.ImageMessage{
                    Caption:    proto.String(caption),
                    Mimetype:   proto.String(mimeType),
                    URL:        proto.String(uploaded.URL),
                    DirectPath: proto.String(uploaded.DirectPath),
                    MediaKey:   uploaded.MediaKey,
                    FileLength: proto.Uint64(uint64(len(fileData))),
                },
            }
        } else {
            // Dokumen atau file lainnya
            uploaded, err := client.Upload(ctx, fileData, whatsmeow.MediaDocument)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document: " + err.Error()})
                return
            }
            
            msg = &waProto.Message{
                DocumentMessage: &waProto.DocumentMessage{
                    Title:      proto.String(fileName),
                    FileName:   proto.String(fileName),
                    Mimetype:   proto.String(mimeType),
                    URL:        proto.String(uploaded.URL),
                    DirectPath: proto.String(uploaded.DirectPath),
                    MediaKey:   uploaded.MediaKey,
                    FileLength: proto.Uint64(uint64(len(fileData))),
                },
            }
        }
        
        // Kirim pesan
        _, err = client.SendMessage(ctx, jid, msg)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        
        // Simpan ke database (opsional, jika ada package database)
        // Note: Jika package database tidak ada, comment baris berikut
        // botJID := client.Store.ID.String()
        // content := fmt.Sprintf("📷 %s", fileName)
        // if caption != "" {
        //     content = content + ": " + caption
        // }
        // database.SaveMessage(botJID, to, content, true)
        
        c.JSON(http.StatusOK, gin.H{
            "status": "sent",
            "to":     to,
            "file":   fileName,
            "type":   strings.Split(mimeType, "/")[0],
        })
    }
}

// GetDeliveryStatus - cek status pengiriman pesan
func GetDeliveryStatus(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        messageID := c.Query("message_id")
        c.JSON(http.StatusOK, gin.H{
            "message_id": messageID,
            "status":     "pending",
            "note":       "Implement with event handler and database",
        })
    }
}