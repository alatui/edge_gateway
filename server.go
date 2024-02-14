package main

import (
	"context"
	"encoding/json"
    "log"
    "net/http"
    "sync"
	"time"
    "github.com/gorilla/websocket"
	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"github.com/go-redis/redis/v8"
)

var (
    upgrader    = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }
    clients     = make(map[string]*websocket.Conn) // Map to track connected clients and their clientIDs
    clientMutex sync.Mutex                         // Mutex to ensure safe access to the clients map
	redisClient = redis.NewClient(&redis.Options{Addr: "localhost:6379", Password: "", DB: 0}) // Redis client
)

type RequestData struct {
    ServiceName     string  `json:"serviceName"`
    ServiceEndpoint string  `json:"serviceEndpoint"`
    HTTPMethod      string  `json:"httpMethod"`
    Payload         string `json:"payload"`
}

type Message struct {
    RequestID     	string  `json:"requestID"`
    ClientID 	string  `json:"clientID"`
    RequestData	RequestData  `json:"requestData"`
}

type ResponseData struct {
	ResponseID	string  `json:"reponseID"`
	Payload     string `json:"payload"`
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
    clientID := r.Header.Get("X-GV-CLIENTID") // Extract clientID from request headers
    if clientID == "" {
        http.Error(w, "X-GV-CLIENTID not provided", http.StatusBadRequest)
        return
    }

    // Upgrade HTTP connection to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal(err)
        return
    }
    defer conn.Close()

    // Lock mutex before accessing clients map
    clientMutex.Lock()
    clients[clientID] = conn
    clientMutex.Unlock()

    // Infinite loop to handle WebSocket messages
    for {
        // Read message from client
        var m ResponseData

		// Read the JSON message from the WebSocket connection
		err := conn.ReadJSON(&m)
		if err != nil {
			log.Fatal("Error reading JSON message:", err)
		}

		key := clientID + ":" + m.ResponseID
		err = redisClient.Set(context.Background(), key, m.Payload, 30*time.Second).Err()
		if err != nil {
			log.Fatal("Error setting Redis message from %s", clientID)
		}
    }

    // Lock mutex before accessing clients map
    clientMutex.Lock()
    delete(clients, clientID) // Remove client from map when connection closes
    clientMutex.Unlock()
}

func tunnelHandler(w http.ResponseWriter, r *http.Request) {
    clientID := r.Header.Get("X-GV-CLIENTID")
	if clientID == "" {
        http.Error(w, "X-GV-CLIENTID not provided", http.StatusBadRequest)
        return
    }

	var requestData RequestData
	err := json.NewDecoder(r.Body).Decode(&requestData)
    if err != nil {
        http.Error(w, "Failed to decode JSON body", http.StatusBadRequest)
        return
    }

	requestID := uuid.NewString()
	message := Message{requestID, clientID, requestData}

    // Retrieve the WebSocket connection for the clientID
    clientMutex.Lock()
    conn, ok := clients[clientID]
    clientMutex.Unlock()

    if !ok {
        http.Error(w, "clientID not found", http.StatusNotFound)
        return
    }

	err = conn.WriteJSON(message)
    if err != nil {
        log.Println("Error sending message to client:", err)
        http.Error(w, "Failed to send message to client", http.StatusInternalServerError)
        return
    }

	result := ""
	key := clientID + ":" + requestID
	for i := 0; i < 100; i++ { //30 seconds timeout
		time.Sleep(300 * time.Millisecond)
		val, err := redisClient.Get(context.Background(), key).Result()
		if err == nil {
			result = val
			break
		}
	}

	if result == "" {
		http.Error(w, "Timeout: result not found", http.StatusInternalServerError)
	}

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(result))
}


func main() {
	// Close the client connection when main returns
	defer redisClient.Close()

    r := mux.NewRouter()

    // WebSocket handler
    r.HandleFunc("/ws", websocketHandler)

    // HTTP GET endpoint for updating clients
    r.HandleFunc("/tunnel", tunnelHandler).Methods("POST")

    // Set up HTTP server with the router
    http.Handle("/", r)

    log.Println("Server starting on localhost:8080")
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
