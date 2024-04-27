package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]string)
var broadcast = make(chan string)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer ws.Close()

	clients[ws] = fmt.Sprintf("Client%d", rand.Intn(100))

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			delete(clients, ws)
			break
		}

		log.Printf("Received message from %s: %s", clients[ws], string(msg))
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func broadcastMessage(message string) {
	broadcast <- message
}

func startServer() {
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				broadcastMessage("Hello clients")
			}
		}
	}()

	log.Println("http server started on :8000")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// Write a startClient function that connects to the server and sends a message "Hello world" to the server periodically
func startClient() {
	c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8000/ws", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("received: %s", message)
		}
	}()

	for {
		select {
		case <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte("Hello server"))
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}

func main() {
	go startServer()

	for i := 0; i < 5; i++ {
		go startClient()
	}

	select {}
}
