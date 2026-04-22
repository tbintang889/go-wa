package handlers

import (
	"context"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
		waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"gowa/database"
	"gowa/utils"
)

func GetQRHandler(client *whatsmeow.Client, qrCode *string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client.IsLoggedIn() {
			c.JSON(200, gin.H{"status": "connected", "message": "Already logged in"})
			return
		}
		if *qrCode == "" {
			c.JSON(200, gin.H{"status": "waiting", "message": "Waiting for QR code..."})
			return
		}
		c.JSON(200, gin.H{"status": "pending", "qr_code": *qrCode})
	}
}

func GetStatusFullHandler(client *whatsmeow.Client, qrCode *string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"is_connected": client.IsConnected(),
			"is_logged_in": client.IsLoggedIn(),
			"qr_available": *qrCode != "",
		})
	}
}
func GetChatsHandler(client *whatsmeow.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Query super simple: ambil semua JID unik
		rows, err := database.DB.Query(`
			SELECT DISTINCT from_jid FROM messages 
			WHERE from_jid != '' AND from_jid != 'me' AND from_jid != to_jid
			UNION
			SELECT DISTINCT to_jid FROM messages 
			WHERE to_jid != '' AND to_jid != 'me' AND from_jid != to_jid
		`)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		chatList := []gin.H{}
		for rows.Next() {
			var jidStr string
			rows.Scan(&jidStr)
			if jidStr == "" || jidStr == "me" {
				continue
			}
			
			// Bersihkan JID untuk display
			displayName := jidStr
			if strings.Contains(jidStr, ":") {
				displayName = strings.Split(jidStr, ":")[0]
			}
			if strings.Contains(displayName, "@") {
				displayName = strings.Split(displayName, "@")[0]
			}
			
			// Push name (optional, tidak ambil last message biar cepat)
			jid, err := types.ParseJID(jidStr)
			if err == nil {
				contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
				if err == nil && contact.PushName != "" {
					displayName = contact.PushName
				}
			}
			
			chatList = append(chatList, gin.H{
				"jid":    jidStr,
				"name":   displayName,
				"number": jidStr,
			})
		}
		
		c.JSON(200, chatList)
	}
}
func GetMessagesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		jidParam := c.Param("jid")
		decodedJID, _ := url.QueryUnescape(jidParam)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		limit := 10
		offset := (page - 1) * limit

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
			if err := rows.Scan(&msg.ID, &msg.FromJID, &msg.ToJID, &msg.Content, &msg.IsFromMe, &msg.Timestamp); err != nil {
				continue
			}
			messages = append(messages, msg)
		}
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		var total int
		database.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE from_jid = ? OR to_jid = ?", decodedJID, decodedJID).Scan(&total)

		c.JSON(200, gin.H{
			"messages":    messages,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"has_more":    total > page*limit,
			"total_pages": (total + limit - 1) / limit,
		})
	}
}

func SendMessageHandler(client *whatsmeow.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		to := utils.NormalizeJID(req.To)
		if !strings.Contains(to, "@") {
			to = to + "@s.whatsapp.net"
		}
		jid, err := types.ParseJID(to)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid JID"})
			return
		}
		botJID := client.Store.ID.String()
		result, err := database.DB.Exec(`INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) VALUES (?, ?, ?, ?, 'pending', ?)`,
			botJID, to, req.Text, true, time.Now())
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to save message"})
			return
		}
		messageID, _ := result.LastInsertId()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if _, err := client.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(req.Text)}); err != nil {
				database.UpdateMessageStatus(int(messageID), "failed")
			} else {
				database.UpdateMessageStatus(int(messageID), "sent")
			}
		}()
		c.JSON(200, gin.H{"status": "pending", "id": messageID})
	}
}

func GetMediaHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var mediaPath string
		err := database.DB.QueryRow("SELECT media_path FROM messages WHERE id = ?", id).Scan(&mediaPath)
		if err != nil || mediaPath == "" {
			c.String(404, "Media not found")
			return
		}
		if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
			c.String(404, "Media file not found")
			return
		}
		c.File(mediaPath)
	}
}

func GetRoutesHandler(r *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		routes := []gin.H{}
		for _, route := range r.Routes() {
			handlerName := runtime.FuncForPC(reflect.ValueOf(route.HandlerFunc).Pointer()).Name()
			routes = append(routes, gin.H{
				"method":  route.Method,
				"path":    route.Path,
				"handler": handlerName,
			})
		}
		c.JSON(200, gin.H{"total_routes": len(routes), "routes": routes})
	}
}

func DebugChatsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := database.DB.Query(`SELECT DISTINCT from_jid FROM messages WHERE from_jid != '' AND from_jid != 'me' UNION SELECT DISTINCT to_jid FROM messages WHERE to_jid != '' AND to_jid != 'me'`)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var jids []string
		for rows.Next() {
			var jid string
			rows.Scan(&jid)
			if jid != "" {
				jids = append(jids, jid)
			}
		}
		c.JSON(200, gin.H{"total": len(jids), "jids": jids, "note": "Raw JIDs from database"})
	}
}