package routes

import (
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"gowa/controllers"
	"gowa/handlers"
)

func SetupRoutes(r *gin.Engine, client *whatsmeow.Client, qrCode *string) {
	// API group
	api := r.Group("/api")
	{
		// Core & Auth
		api.GET("/qr", handlers.GetQRHandler(client, qrCode))
		api.GET("/status", controllers.Status(client))
		api.GET("/status-full", handlers.GetStatusFullHandler(client, qrCode))

		// Messaging
		api.POST("/send-text", controllers.SendText(client))
		api.POST("/send-message", handlers.SendMessageHandler(client))
		api.POST("/send-media", controllers.SendMedia(client))
		api.GET("/delivery-status", controllers.GetDeliveryStatus(client))
		api.GET("/messages/:jid", handlers.GetMessagesHandler())

		// Data & Contacts
		api.GET("/chats", handlers.GetChatsHandler(client))
		api.GET("/contacts", controllers.GetContacts(client))

		// Groups
		api.GET("/groups", controllers.GetGroups(client))
		api.GET("/group-members", controllers.GetGroupMembers(client))

		// Media
		api.GET("/media/:id", handlers.GetMediaHandler())

		// Debug
		api.GET("/debug/chats", handlers.DebugChatsHandler())
		api.GET("/routes", handlers.GetRoutesHandler(r))
	}

	// Webhook
	webhook := r.Group("/webhook")
	{
		webhook.POST("/incoming", controllers.IncomingWebhook)
	}

	// UI Pages
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{})
	})
	r.GET("/chat", func(c *gin.Context) {
		c.HTML(200, "chat.html", gin.H{})
	})
}