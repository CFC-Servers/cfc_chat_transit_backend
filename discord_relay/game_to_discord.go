package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

var allowedRealms = map[string]bool{
	"cfc3": true,
	"cfcttt": true,
}

// load from env?
var realmSecrets = map[string]string{
	"cfc3":  "cfc3_secret",
	"cfcttt": "cfcttt_secret",
}

type wsConnection struct {
	conn     *websocket.Conn
	outgoing chan []byte
}

var wsConnections = make(map[string]*wsConnection)
var wsConnectionsMutex = &sync.Mutex{}

func handleRead(wsConn *wsConnection, realm string, r *http.Request) {
	c := wsConn.conn
	defer func() {
		c.Close()
		wsConnectionsMutex.Lock()
		if wsConnections[realm] == wsConn {
			delete(wsConnections, realm)
		}
		wsConnectionsMutex.Unlock()
	}()

	// Start keep-alive
	go keepAlive(c, r)

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		select {
		case MessageQueue <- message:
			// Message added to queue
		default:
			log.Print("Queue was full, could not add message!", string(message))
		}
	}
}

func handleWrite(wsConn *wsConnection) {
	c := wsConn.conn

    for msg := range wsConn.outgoing {
        err := c.WriteMessage(websocket.TextMessage, msg)
        if err != nil {
            log.Println("write:", err)
            return
        }
    }
}

func keepAlive(c *websocket.Conn, r *http.Request) {
	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := c.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			if err != nil {
				log.Print("Received an error when sending keepalive. Exiting keepalive loop")
				return
			}
		case <-ctx.Done():
			log.Print("Request context is done. Exiting keepalive loop")
			return
		}
	}
}

func relay(w http.ResponseWriter, r *http.Request) {
	// Get realm from query parameter
	realm := r.URL.Query().Get("realm")
	if realm == "" {
		log.Println("No realm provided")
		http.Error(w, "No realm provided", http.StatusBadRequest)
		return
	}

	if !allowedRealms[realm] {
		log.Printf("Realm '%s' is not allowed", realm)
		http.Error(w, "Realm not allowed", http.StatusForbidden)
		return
	}

	authToken := r.URL.Query().Get("auth_token")
	expectedToken := realmSecrets[realm]

	if authToken != expectedToken {
		log.Printf("Invalid auth token for realm '%s'", realm)
		http.Error(w, "Invalid auth token", http.StatusForbidden)
		return
	}

	if wsConnections[realm] != nil {
		log.Printf("Connection for realm '%s' already exists", realm)
		http.Error(w, "Connection already exists", http.StatusConflict)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade err:", err)
		return
	}

	wsConn := &wsConnection{
		conn:     c,
		outgoing: make(chan []byte, 256),
	}

	wsConnectionsMutex.Lock()
	wsConnections[realm] = wsConn
	wsConnectionsMutex.Unlock()

	go handleRead(wsConn, realm, r)
	go handleWrite(wsConn)
}
