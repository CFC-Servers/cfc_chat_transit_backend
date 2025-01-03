package main

import (
	"encoding/json"
	"net/http"
)

func sendToServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type ServerMessage struct {
		Realm      string `json:"realm"`
		SenderName string `json:"sender_name"`
		ColorRGB   string `json:"color_rgb"` // Format: "R,G,B"
		Content    string `json:"content"`
	}

	var msg ServerMessage
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if msg.Realm == "" || msg.Content == "" || msg.SenderName == "" || msg.ColorRGB == "" {
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

	// Validate ColorRGB format
	colorComponents := strings.Split(msg.ColorRGB, ",")
	if len(colorComponents) != 3 {
		http.Error(w, "Invalid color_rgb format. Expected format: R,G,B", http.StatusBadRequest)
		return
	}

	// Create the websocket message
	messageToSend := WebsocketMessage{
		SenderName: msg.SenderName,
		ColorRGB:   msg.ColorRGB,
		Content:    msg.Content,
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
type WebsocketMessage struct {
	SenderName string `json:"sender_name"`
	ColorRGB   string `json:"color_rgb"`
	Content    string `json:"content"`
}
