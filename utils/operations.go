package utils

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/RazerFrFr/Voryn/structs"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const XMPPDomain string = "prod.ol.epicgames.com"

func HandleOpen(client *structs.Client, data map[string]string, server *structs.Server) {
	if _, ok := data["ID"]; !ok {
		id := uuid.New()
		data["ID"] = strings.ReplaceAll(id.String(), "-", "")
	}

	openXML := fmt.Sprintf(`<open xmlns="urn:ietf:params:xml:ns:xmpp-framing" from="%s" id="%s" version="1.0" xml:lang="en"/>`,
		XMPPDomain, data["ID"])
	client.Conn.WriteMessage(websocket.TextMessage, []byte(openXML))

	var response string
	if client.Authenticated {
		response = `<stream:features xmlns:stream="http://etherx.jabber.org/streams">
			<ver xmlns="urn:xmpp:features:rosterver"/>
			<starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"/>
			<bind xmlns="urn:ietf:params:xml:ns:xmpp-bind"/>
			<compression xmlns="http://jabber.org/features/compress">
				<method>zlib</method>
			</compression>
			<session xmlns="urn:ietf:params:xml:ns:xmpp-session"/>
		</stream:features>`
	} else {
		response = `<stream:features xmlns:stream="http://etherx.jabber.org/streams">
			<mechanisms xmlns="urn:ietf:params:xml:ns:xmpp-sasl">
				<mechanism>PLAIN</mechanism>
			</mechanisms>
			<ver xmlns="urn:xmpp:features:rosterver"/>
			<starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"/>
			<compression xmlns="http://jabber.org/features/compress">
				<method>zlib</method>
			</compression>
			<auth xmlns="http://jabber.org/features:iq-auth"/>
		</stream:features>`
	}

	client.Conn.WriteMessage(websocket.TextMessage, []byte(response))
}

func HandleAuth(client *structs.Client, content string, server *structs.Server) error {
	if client.AccountID != "" {
		return nil
	}
	if content == "" {
		Logger.Error("Auth content missing")
		SendSASLError(client, "not-authorized")
		return fmt.Errorf("content missing")
	}

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		Logger.Error("Base64 decode failed:", err)
		SendSASLError(client, "not-authorized")
		return err
	}

	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		Logger.Error("Decoded auth parts invalid")
		SendSASLError(client, "not-authorized")
		return fmt.Errorf("invalid auth format")
	}

	tokenStr := parts[2]
	tokenStore, err := FindToken(tokenStr)
	if err != nil {
		Logger.Error("Access token not found:", err)
		SendSASLError(client, "not-authorized")
		return fmt.Errorf("invalid token")
	}

	accountID := tokenStore.AccountID

	for _, c := range server.Clients {
		if c.AccountID == accountID {
			Logger.Error("Client already connected")
			SendSASLError(client, "conflict")
			return fmt.Errorf("already connected")
		}
	}

	user, err := GetUserByAccountID(accountID)
	if err != nil || user.Banned {
		Logger.Error("User not found or banned")
		SendSASLError(client, "not-authorized")
		return fmt.Errorf("invalid user")
	}

	client.AccountID = user.AccountID
	client.DisplayName = user.Username
	client.Token = tokenStr
	client.Authenticated = true

	successXML := `<success xmlns="urn:ietf:params:xml:ns:xmpp-sasl"/>`
	client.Conn.WriteMessage(websocket.TextMessage, []byte(successXML))
	return nil
}

func HandleIQ(client *structs.Client, root map[string]interface{}, server *structs.Server) {
	id, _ := root["-id"].(string)
	iqType, _ := root["-type"].(string)
	if id == "" || iqType != "set" {
		return
	}

	switch id {
	case "_xmpp_bind1":
		if client.AccountID == "" {
			SendError(client)
			return
		}

		if client.Resource != "" {
			return
		}

		bindNode, ok := root["bind"].(map[string]interface{})
		if !ok {
			SendError(client)
			return
		}

		resource, _ := bindNode["resource"].(string)
		if resource == "" {
			SendError(client)
			return
		}

		client.Resource = resource
		client.JID = fmt.Sprintf("%s@%s/%s", client.AccountID, XMPPDomain, client.Resource)

		bindXML := fmt.Sprintf(`<iq to="%s" id="_xmpp_bind1" type="result" xmlns="jabber:client">
			<bind xmlns="urn:ietf:params:xml:ns:xmpp-bind"><jid>%s</jid></bind>
		</iq>`, client.JID, client.JID)

		client.Conn.WriteMessage(websocket.TextMessage, []byte(bindXML))

	case "_xmpp_session1":
		if client.AccountID == "" || client.Resource == "" {
			SendError(client)
			return
		}

		sessionXML := fmt.Sprintf(`<iq to="%s" from="%s" id="_xmpp_session1" type="result" xmlns="jabber:client"/>`,
			client.JID, XMPPDomain)
		client.Conn.WriteMessage(websocket.TextMessage, []byte(sessionXML))

	default:
		if client.AccountID == "" || client.Resource == "" {
			SendError(client)
			return
		}
		iqXML := fmt.Sprintf(`<iq to="%s" from="%s" id="%s" type="result" xmlns="jabber:client"/>`,
			client.JID, XMPPDomain, id)
		client.Conn.WriteMessage(websocket.TextMessage, []byte(iqXML))
	}
}

func HandlePresence(client *structs.Client, msg map[string]interface{}, server *structs.Server) {
	if !client.ClientExists {
		SendError(client)
		return
	}

	presenceType, _ := msg["@_type"].(string)
	to, _ := msg["@_to"].(string)
	status, _ := msg["status"].(string)
	away := msg["show"] != nil

	switch presenceType {
	case "unavailable":
		roomName := strings.Split(to, "@")[0]
		if roomMembers, ok := server.MUCs[roomName]; ok {
			delete(roomMembers, client.AccountID)
			for i, rn := range client.JoinedMUCs {
				if rn == roomName {
					client.JoinedMUCs = append(client.JoinedMUCs[:i], client.JoinedMUCs[i+1:]...)
					break
				}
			}
		}

	default:
		roomName := strings.Split(to, "@")[0]
		if _, ok := server.MUCs[roomName]; !ok {
			server.MUCs[roomName] = make(map[string]string)
		}
		server.MUCs[roomName][client.AccountID] = client.DisplayName
		client.JoinedMUCs = append(client.JoinedMUCs, roomName)
	}

	client.LastPresenceUpdate.Status = status
	client.LastPresenceUpdate.Away = away
	UpdatePresenceForFriends(server, client, status, away, presenceType == "unavailable")
}
