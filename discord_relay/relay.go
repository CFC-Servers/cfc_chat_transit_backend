package main

import (
	"log"
	"os"
	"strings"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsConnection struct {
	conn     *websocket.Conn
	outgoing chan []byte
}

func handleRead(wsConn *wsConnection, serverId string, r *http.Request) {
	c := wsConn.conn
	defer func() {
		c.Close()
		wsConnectionsMutex.Lock()
		if gtdWsConnections[realm] == wsConn {
			delete(gtdWsConnections, realm)
		}
		gtdWsConnectionsMutex.Unlock()
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

	for {
		select {
		case msg, ok := <-wsConn.outgoing:
			if !ok {
				// Channel closed
				return
			}

			err := c.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}

var upgrader = websocket.Upgrader{}
var allowedRealms = make(map[string]bool)
var realmSecrets = make(map[string]string)
var dtgWsConnections = make(map[string]*wsConnection)

func loadAllowedRealms() {
	realms := os.Getenv("ALLOWED_REALMS")
	if realms == "" {
		log.Fatal("ALLOWED_REALMS environment variable not set")
	}
	for _, realm := range strings.Split(realms, ",") {
		realm = strings.TrimSpace(realm)
		allowedRealms[realm] = true

		// Load the secret for the realm
		secretEnvVar := fmt.Sprintf("REALM_%s_SECRET", strings.ToUpper(realm))
		secret := os.Getenv(secretEnvVar)
		if secret == "" {
			log.Fatalf("Secret for realm '%s' not set in environment variable '%s'", realm, secretEnvVar)
		}
		realmSecrets[realm] = secret
	}
}
var dtgWsConnectionsMutex = &sync.Mutex{}

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
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	// Get realm from query parameter
	realm := r.URL.Query().Get("realm")
	if realm == "" {
		log.Println("No realm provided")
		c.Close()
		return
	}

	if !allowedRealms[realm] {
		log.Printf("Realm '%s' is not allowed", realm)
		c.Close()
		return
	}

	authToken := r.URL.Query().Get("auth_token")
	expectedToken := realmSecrets[realm]

	if authToken != expectedToken {
		log.Printf("Invalid auth token for realm '%s'", realm)
		c.Close()
		return
	}

	wsConn := &wsConnection{
		conn:     c,
		outgoing: make(chan []byte, 256),
	}

	// Store the connection in the map
	wsConnectionsMutex.Lock()
	if existingConn, exists := gtdWsConnections[realm]; exists {
		log.Printf("Existing connection for realm '%s' found, closing it", realm)
		existingConn.conn.Close()
	}
	gtdWsConnections[realm] = wsConn
	gtdWsConnectionsMutex.Unlock()

	// Start goroutines for handling the connection
	go handleRead(wsConn, serverId, r)
	go handleWrite(wsConn)
}
