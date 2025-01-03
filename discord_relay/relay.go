package main

import (
	"log"
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
		delete(wsConnections, serverId)
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
var wsConnections = make(map[string]*wsConnection)
var wsConnectionsMutex = &sync.Mutex{}

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

	// Get server ID from query parameter
	serverId := r.URL.Query().Get("server_id")
	if serverId == "" {
		log.Println("No server ID provided")
		c.Close()
		return
	}

	wsConn := &wsConnection{
		conn:     c,
		outgoing: make(chan []byte, 256),
	}

	// Store the connection in the map
	wsConnectionsMutex.Lock()
	wsConnections[serverId] = wsConn
	wsConnectionsMutex.Unlock()

	// Start goroutines for handling the connection
	go handleRead(wsConn, serverId, r)
	go handleWrite(wsConn)
}
