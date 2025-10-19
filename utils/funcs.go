package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/RazerFrFr/Voryn/models"
	"github.com/RazerFrFr/Voryn/structs"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Database

func InitDB() {
	uri := os.Getenv("MONGO_URI")
	dbName := os.Getenv("DB_NAME")

	if uri == "" || dbName == "" {
		log.Fatal("MONGO_URI or DB_NAME environment variable not set")
	}

	fullURI := fmt.Sprintf("%s%s", uri, dbName)

	clientOptions := options.Client().
		ApplyURI(fullURI).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		Logger.Error("Failed to create MongoDB client:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		Logger.Error("MongoDB connection failed:", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		Logger.Error("MongoDB ping failed:", err)
	}

	DB = client.Database(dbName)
	Logger.MongoDB(fmt.Sprintf("Connection to %s successfully established.", fullURI))
}

func SendError(client *structs.Client) {
	closeXML := `<close xmlns="urn:ietf:params:xml:ns:xmpp-framing"/>`
	client.Conn.WriteMessage(1, []byte(closeXML))
	client.Conn.Close()
}

func GetUserByAccountID(accountID string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := DB.Collection("users")

	var user models.User
	err := collection.FindOne(ctx, bson.M{"accountId": accountID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if user.Banned {
		return nil, fmt.Errorf("user is banned")
	}

	return &user, nil
}

func RemoveClient(server *structs.Server, client *structs.Client) {
	server.ClientsMutex.Lock()
	for i, c := range server.Clients {
		if c == client {
			server.Clients = append(server.Clients[:i], server.Clients[i+1:]...)
			break
		}
	}
	server.ClientsMutex.Unlock()

	UpdatePresenceForFriends(server, client, "{}", false, true)

	partyID := ""
	var clientStatus map[string]interface{}
	if err := json.Unmarshal([]byte(client.LastPresenceUpdate.Status), &clientStatus); err == nil {
		if props, ok := clientStatus["Properties"].(map[string]interface{}); ok {
			for key, val := range props {
				if len(key) >= 14 && strings.ToLower(key[:14]) == "party.joininfo" {
					if obj, ok := val.(map[string]interface{}); ok {
						if pid, ok := obj["partyId"].(string); ok {
							partyID = pid
							break
						}
					}
				}
			}
		}
	}

	if partyID != "" {
		for _, c := range server.Clients {
			if c.AccountID == client.AccountID {
				continue
			}

			go func(c *structs.Client) {
				msg := map[string]interface{}{
					"type": "com.epicgames.party.memberexited",
					"payload": map[string]interface{}{
						"partyId":   partyID,
						"memberId":  client.AccountID,
						"wasKicked": false,
					},
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}
				data, _ := json.Marshal(msg)
				xmlMsg := fmt.Sprintf(`<message from="%s" to="%s"><body>%s</body></message>`,
					client.JID, c.JID, string(data))

				c.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := c.Conn.WriteMessage(websocket.TextMessage, []byte(xmlMsg)); err != nil {
					Logger.Error("Failed to send party exit:", err)
				}
			}(c)
		}
	}

	client.AccountID = ""
	client.JID = ""
	client.Resource = ""
	client.Token = ""
	client.Authenticated = false
	client.ClientExists = false
}

func GetFriendsClients(server *structs.Server, accountID string) ([]*structs.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := DB.Collection("users")

	var friendsDoc models.Friends
	err := collection.FindOne(ctx, bson.M{"accountId": accountID}).Decode(&friendsDoc)
	if err != nil {
		return nil, err
	}

	var friendsClients []*structs.Client

	for _, f := range friendsDoc.List.Accepted {
		for _, c := range server.Clients {
			if c.AccountID == f.AccountID {
				friendsClients = append(friendsClients, c)
				break
			}
		}
	}

	return friendsClients, nil
}

func GetFriendsPresence(server *structs.Server, ws *structs.Client, friends []*structs.Client) {
	for _, friend := range friends {
		presenceXML := fmt.Sprintf(`<presence from="%s" to="%s" type="available"><status>%s</status></presence>`,
			friend.JID, ws.JID, friend.LastPresenceUpdate.Status)
		if friend.LastPresenceUpdate.Away {
			presenceXML = fmt.Sprintf(`<presence from="%s" to="%s" type="available"><show>away</show><status>%s</status></presence>`,
				friend.JID, ws.JID, friend.LastPresenceUpdate.Status)
		}
		ws.Conn.WriteMessage(1, []byte(presenceXML))
	}
}

func UpdatePresenceForFriends(server *structs.Server, sender *structs.Client, body string, away, offline bool) {
	sender.LastPresenceUpdate.Away = away
	sender.LastPresenceUpdate.Status = body

	server.ClientsMutex.Lock()
	defer server.ClientsMutex.Unlock()

	for _, client := range server.Clients {
		if client.AccountID == sender.AccountID {
			continue
		}
		presenceType := "available"
		if offline {
			presenceType = "unavailable"
		}
		statusXML := body
		if away {
			presenceXML := fmt.Sprintf(`<presence from="%s" to="%s" type="%s"><show>away</show><status>%s</status></presence>`,
				sender.JID, client.JID, presenceType, statusXML)
			client.Conn.WriteMessage(1, []byte(presenceXML))
		} else {
			presenceXML := fmt.Sprintf(`<presence from="%s" to="%s" type="%s"><status>%s</status></presence>`,
				sender.JID, client.JID, presenceType, statusXML)
			client.Conn.WriteMessage(1, []byte(presenceXML))
		}
	}
}

func FindClientByAccountID(server *structs.Server, accountID string) *structs.Client {
	server.ClientsMutex.Lock()
	defer server.ClientsMutex.Unlock()

	for _, c := range server.Clients {
		if c.AccountID == accountID {
			return c
		}
	}
	return nil
}

func SendSASLError(client *structs.Client, condition string) {
	errXML := fmt.Sprintf(
		`<failure xmlns="urn:ietf:params:xml:ns:xmpp-sasl"><%s/></failure>`, condition,
	)
	client.Conn.WriteMessage(websocket.TextMessage, []byte(errXML))
}

type TokenPayload struct {
	App          string `json:"app,omitempty"`
	Sub          string `json:"sub,omitempty"`
	Dvid         string `json:"dvid,omitempty"`
	Mver         bool   `json:"mver,omitempty"`
	Clid         string `json:"clid,omitempty"`
	Dn           string `json:"dn,omitempty"`
	Am           string `json:"am,omitempty"`
	P            string `json:"p,omitempty"`
	Iai          string `json:"iai,omitempty"`
	Sec          int    `json:"sec,omitempty"`
	Clsvc        string `json:"clsvc,omitempty"`
	T            string `json:"t,omitempty"`
	Ic           bool   `json:"ic,omitempty"`
	Jti          string `json:"jti,omitempty"`
	CreationDate string `json:"creation_date"`
	HoursExpire  int    `json:"hours_expire"`
}

func DecodeToken(token string) (*TokenPayload, error) {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "eg1~")

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload decode error: %w", err)
	}

	var claims TokenPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	creationTime, err := time.Parse(time.RFC3339, claims.CreationDate)
	if err != nil {
		return nil, fmt.Errorf("invalid creation_date: %w", err)
	}
	expiry := creationTime.Add(time.Duration(claims.HoursExpire) * time.Hour)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}
