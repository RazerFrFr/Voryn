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
}

type Server struct {
	Clients      []*Client
	ClientsMutex sync.Mutex
}
