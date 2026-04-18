package routes

import (
    "github.com/gin-gonic/gin"
    "go.mau.fi/whatsmeow"
    "gowa/controllers"
)

func WaRoutes(r *gin.Engine, client *whatsmeow.Client) {
    api := r.Group("/api")
    {
        // Core
        api.GET("/status", controllers.Status(client))
        
        // Messaging
        api.POST("/send-text", controllers.SendText(client))
        api.POST("/send-media", controllers.SendMedia(client))
        api.GET("/delivery-status", controllers.GetDeliveryStatus(client))
        
        // Data & Contacts
        api.GET("/contacts", controllers.GetContacts(client))
        
        // Groups
        api.GET("/groups", controllers.GetGroups(client))
        api.GET("/group-members", controllers.GetGroupMembers(client))
    }

    webhook := r.Group("/webhook")
    {
        webhook.POST("/incoming", controllers.IncomingWebhook)
    }
}