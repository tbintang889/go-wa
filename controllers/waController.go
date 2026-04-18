package controllers

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "google.golang.org/protobuf/proto"
)

func Status(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": client.IsConnected()})
    }
}

func GetContacts(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        contacts, _ := client.Store.Contacts.GetAllContacts(context.Background())
        c.JSON(http.StatusOK, contacts)
    }
}

func GetGroups(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        groups, err := client.GetJoinedGroups(context.Background())
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, groups)
    }
}

func GetGroupMembers(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        groupID := c.Query("group_id")
        jid := types.JID{User: groupID, Server: "g.us"}

        info, err := client.GetGroupInfo(context.Background(), jid)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, info.Participants)
    }
}

/* func SendText(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            To   string `json:"to" form:"to"`
            Text string `json:"text" form:"text"`
        }
        if err := c.ShouldBind(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        jid := types.JID{User: req.To, Server: "s.whatsapp.net"}
        _, err := client.SendMessage(context.Background(), jid, &waProto.Message{
            Conversation: proto.String(req.Text),
        })
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{"status": "sent", "to": req.To, "text": req.Text})
    }
} */

func SendText(client *whatsmeow.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            To   string `json:"to" form:"to"`
            Text string `json:"text" form:"text"`
        }
        if err := c.ShouldBind(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        jid := types.JID{User: req.To, Server: "s.whatsapp.net"}
        _, err := client.SendMessage(context.Background(), jid, &waProto.Message{
            Conversation: proto.String(req.Text),
        })
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{"status": "sent", "to": req.To, "text": req.Text})
    }
}

func IncomingWebhook(c *gin.Context) {
    var msg struct {
        Sender    string `json:"sender"`
        Message   string `json:"message"`
        Timestamp string `json:"timestamp"`
    }
    if err := c.ShouldBindJSON(&msg); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // TODO: simpan ke CRM
    c.JSON(http.StatusOK, gin.H{"status": "received"})
}
