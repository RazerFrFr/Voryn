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
	godotenv.Load()
	utils.InitDB()

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	xmppServer := &structs.Server{
		Clients:      []*structs.Client{},
		MUCs:         make(map[string]map[string]string),
		AccessTokens: utils.LoadAccessTokens(),
	}

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Voryn, Made by Razer.")
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

	utils.Logger.XMPP("XMPP server started on port", port)

	// Handle WebSocket and HTTP at the same time (this is the working way that ik ig)
	httpServer := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if websocket.IsWebSocketUpgrade(req) {
				ws, err := upgrader.Upgrade(w, req, nil)
				if err != nil {
					utils.Logger.Error("WebSocket upgrade failed:", err)
					return
				}
				client := &structs.Client{Conn: ws}
				handleWebsocket(ws, client, xmppServer)
				return
			}

			r.ServeHTTP(w, req)
		}),
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		utils.Logger.Error("Server failed:", err)
	}
}

func handleWebsocket(ws *websocket.Conn, client *structs.Client, server *structs.Server) {
	client.JoinedMUCs = []string{}

	server.ClientsMutex.Lock()
	server.Clients = append(server.Clients, client)
	client.ClientExists = true
	server.ClientsMutex.Unlock()

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			utils.Logger.Error("WebSocket read error:", err)
			utils.RemoveClient(server, client)
			break
		}

		msgStr := string(message)
		msgStr = strings.TrimSpace(msgStr)
		if msgStr == "" {
			continue
		}

		var root mxj.Map
		root, err = mxj.NewMapXml([]byte(msgStr))
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
				data := map[string]string{}
				utils.HandleOpen(client, data, server)

			case "auth":
				content := ""
				if v, ok := nodeValue.(map[string]interface{})["#text"]; ok {
					content, _ = v.(string)
				}
				utils.HandleAuth(client, content, server)

			case "iq":
				nodeMap, ok := nodeValue.(map[string]interface{})
				if ok {
					utils.HandleIQ(client, nodeMap, server)
				}

			case "presence":
				nodeMap, ok := nodeValue.(map[string]interface{})
				if ok {
					utils.HandlePresence(client, nodeMap, server)
				}

			case "close":
				utils.RemoveClient(server, client)
				ws.Close()
				return
			}
		}

		if !client.ClientExists && client.AccountID != "" && client.DisplayName != "" && client.Token != "" && client.JID != "" && client.Resource != "" && client.Authenticated {
			server.ClientsMutex.Lock()
			server.Clients = append(server.Clients, client)
			client.ClientExists = true
			server.ClientsMutex.Unlock()
		}
	}
}
