package main

import (
	"encoding/json"
	"net/http"
	"os"
)

type RGB struct {
    R uint8 `json:"r"`
    G uint8 `json:"g"`
    B uint8 `json:"b"`
}

// What we send to the game server
type DiscordToGameWebsocketMessage struct {
    MessageType string `json:"message_type"`
	SenderName string `json:"sender_name"`
	ColorRGB   RGB `json:"color_rgb"`
	Content    string `json:"content"`
}

// What we receive
type DiscordToGameMessage struct {
    Realm      string `json:"realm"`
    SenderName string `json:"sender_name"`
    ColorRGB   RGB `json:"color_rgb"`
    Content    string `json:"content"`
}

func sendToGameServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check the auth header
    senderAuth := os.Getenv("FROM_DISCORD_AUTH")
	if r.Header.Get("Authorization") != senderAuth {
	    http.Error(w, "Unauthorized", http.StatusUnauthorized)
	    return
    }

	var msg DiscordToGameMessage
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if msg.Realm == "" || msg.Content == "" || msg.SenderName == "" {
		http.Error(w, "Missing realm, sender_name, color_rgb, or content", http.StatusBadRequest)
		return
	}

	// Retrieve the websocket connection for the realm
	wsConnectionsMutex.Lock()
	wsConn, ok := wsConnections[msg.Realm]
	wsConnectionsMutex.Unlock()

	if !ok {
		http.Error(w, "Server not connected", http.StatusNotFound)
		return
	}

	// Create the websocket message
	messageToSend := DiscordToGameWebsocketMessage{
        MessageType: "chat",
		SenderName: msg.SenderName,
        ColorRGB: msg.ColorRGB,
		Content: msg.Content,
	}

	// Serialize the message to JSON
	messageBytes, err := json.Marshal(messageToSend)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Send the message to the game server via the websocket
	select {
	case wsConn.outgoing <- messageBytes:
		// Message sent successfully
		w.WriteHeader(http.StatusOK)
	default:
		// The outgoing channel is full
		http.Error(w, "Server message buffer is full", http.StatusInternalServerError)
	}
}
