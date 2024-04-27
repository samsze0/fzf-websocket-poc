package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
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

var apiKey = "poc"

func handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("API-KEY") != apiKey {
		w.WriteHeader(http.StatusUnauthorized) // 401
		return
	}

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

func startServer(port ...int) (int, error) {
	var listener net.Listener
	var err error

	if len(port) > 0 {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port[0]))
	} else {
		listener, err = net.Listen("tcp", ":0") // let system choose random port
	}

	if err != nil {
		return 0, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleConnections)

	go handleMessages()

	ticker := time.NewTicker(time.Second * 1)

	go func() {
		for {
			select {
			case <-ticker.C:
				broadcastMessage("Hello clients")
			}
		}
	}()

	log.Println("http server started on", listener.Addr().(*net.TCPAddr).Port)

	go func() {
		err := http.Serve(listener, mux)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
		listener.Close()
	}()

	return listener.Addr().(*net.TCPAddr).Port, nil
}

func startClient(port int) {
	dialer := websocket.DefaultDialer

	req, err := http.NewRequest("GET", fmt.Sprintf("ws://localhost:%d/ws", port), nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("API-KEY", apiKey)

	c, _, err := dialer.DialContext(context.Background(), req.URL.String(), req.Header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	ticker := time.NewTicker(time.Second * 1)
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
	port, err := startServer()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		go startClient(port)
	}

	select {}
}
