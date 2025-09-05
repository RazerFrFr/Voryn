package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/RazerFrFr/Voryn/structs"
	"github.com/RazerFrFr/Voryn/utils"
	"github.com/clbanning/mxj/v2"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	_ = godotenv.Load()
	utils.InitDB()

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	xmppServer := &structs.Server{
		Clients: []*structs.Client{},
	}

	r := gin.Default()
	r.RedirectTrailingSlash = false

	r.Use(func(c *gin.Context) {
		if (c.Request.URL.Path == "/" || c.Request.URL.Path == "//") && websocket.IsWebSocketUpgrade(c.Request) {
			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				utils.Logger.Error("WebSocket upgrade failed:", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "WebSocket upgrade failed"})
				return
			}

			client := &structs.Client{Conn: ws}
			go handleWebsocket(ws, client, xmppServer)

			c.Abort()
			return
		}

		c.Next()
	})

	r.GET("/", func(c *gin.Context) {
		c.String(200, "Voryn, Made by Razer.")
	})

	r.GET("/clients", func(c *gin.Context) {
		xmppServer.ClientsMutex.Lock()
		defer xmppServer.ClientsMutex.Unlock()
		names := []string{}
		for _, cl := range xmppServer.Clients {
			names = append(names, cl.DisplayName)
		}
		c.JSON(http.StatusOK, gin.H{
			"usersAmount": len(xmppServer.Clients),
			"Clients":     names,
		})
	})

	r.POST("/api/voryn/message/send/:accountId", func(c *gin.Context) {
		accountID := c.Param("accountId")

		var body interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}

		if err := utils.SendMessage(body, accountID, xmppServer); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.Status(204)
	})

	r.POST("/api/voryn/presence/send/:accountId/:receiverId", func(c *gin.Context) {
		accountID := c.Param("accountId")
		receiverID := c.Param("receiverId")
		offlineQuery := c.Query("offline")

		isOffline := false
		if offlineQuery == "true" {
			isOffline = true
		}

		if err := utils.SendPresence(accountID, receiverID, isOffline, xmppServer); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.Status(204)
	})

	utils.Logger.XMPP("XMPP server started on port", port)
	if err := r.Run(":" + port); err != nil {
		utils.Logger.Error("Server failed:", err)
	}
}

func handleWebsocket(ws *websocket.Conn, client *structs.Client, server *structs.Server) {
	server.ClientsMutex.Lock()
	server.Clients = append(server.Clients, client)
	client.ClientExists = true
	server.ClientsMutex.Unlock()

	defer func() {
		utils.RemoveClient(server, client)
		_ = ws.Close()
	}()

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			utils.Logger.Error("WebSocket read error:", err)
			return
		}

		msgStr := strings.TrimSpace(string(message))
		if msgStr == "" {
			continue
		}

		root, err := mxj.NewMapXml([]byte(msgStr))
		if err != nil {
			utils.Logger.Error("Failed to parse XML:", err)
			utils.SendError(client)
			continue
		}

		for nodeName, nodeValue := range root {
			baseName := nodeName
			if strings.Contains(nodeName, ":") {
				parts := strings.Split(nodeName, ":")
				baseName = parts[1]
			}

			switch baseName {
			case "open":
				utils.HandleOpen(client, map[string]string{}, server)
			case "auth":
				content := ""
				if v, ok := nodeValue.(map[string]interface{})["#text"]; ok {
					content, _ = v.(string)
				}
				utils.HandleAuth(client, content, server)
			case "iq":
				if nodeMap, ok := nodeValue.(map[string]interface{}); ok {
					utils.HandleIQ(client, nodeMap, server)
				}
			case "presence":
				if nodeMap, ok := nodeValue.(map[string]interface{}); ok {
					utils.HandlePresence(client, nodeMap, server)
				}
			case "close":
				return
			}
		}
	}
}
