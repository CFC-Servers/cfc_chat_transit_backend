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
		ServerId string `json:"server_id"`
		Content  string `json:"content"`
	}

	var msg ServerMessage
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if msg.ServerId == "" || msg.Content == "" {
		http.Error(w, "Missing server_id or content", http.StatusBadRequest)
		return
	}

	// Retrieve the websocket connection for the server
	wsConnectionsMutex.Lock()
	wsConn, ok := wsConnections[msg.ServerId]
	wsConnectionsMutex.Unlock()

	if !ok {
		http.Error(w, "Server not connected", http.StatusNotFound)
		return
	}

	// Send the message to the game server via the websocket connection
	select {
	case wsConn.outgoing <- []byte(msg.Content):
		// Message sent successfully
		w.WriteHeader(http.StatusOK)
	default:
		// The outgoing channel is full
		http.Error(w, "Server message buffer is full", http.StatusInternalServerError)
	}
}