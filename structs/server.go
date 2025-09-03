package structs

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn               *websocket.Conn
	JID                string
	AccountID          string
	DisplayName        string
	Token              string
	Resource           string
	LastPresenceUpdate struct {
		Away   bool
		Status string
	}
	Authenticated    bool
	ClientExists     bool
	ConnectionClosed bool
	JoinedMUCs       []string
}

type Server struct {
	Clients      []*Client
	ClientsMutex sync.Mutex
	MUCs         map[string]map[string]string
	AccessTokens map[string]string
}
