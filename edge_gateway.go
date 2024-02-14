package main

import (
	"encoding/json"
    "log"
    "net/http"
    "sync"
	"time"
    "github.com/gorilla/websocket"
	"github.com/gorilla/mux"
	"github.com/google/uuid"
)

var (
    upgrader    = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }
    agents     = make(map[string]*websocket.Conn) // Map to track connected agents and their agentIDs
    agentMutex sync.Mutex                         // Mutex to ensure safe access to the agents map
    responses  = make(map[string]string) // Map to track responses from agents
    responseMutex sync.Mutex              // Mutex to ensure safe access to the responses map
)

type RequestData struct {
    ServiceName     string  `json:"serviceName"`
    ServiceEndpoint string  `json:"serviceEndpoint"`
    HTTPMethod      string  `json:"httpMethod"`
    Payload         string `json:"payload"`
}

type Message struct {
    RequestID     	string  `json:"requestID"`
    AgentID 	string  `json:"agentID"`
    RequestData	RequestData  `json:"requestData"`
}

type ResponseData struct {
	ResponseID	string  `json:"reponseID"`
	Payload     string `json:"payload"`
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
    agentID := r.Header.Get("AGENT-ID") // Extract agentID from request headers
    if agentID == "" {
        http.Error(w, "AGENT-ID not provided", http.StatusBadRequest)
        return
    }

    // Upgrade HTTP connection to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal(err)
        return
    }
    defer conn.Close()

    // Lock mutex before accessing agents map
    agentMutex.Lock()
    agents[agentID] = conn
    agentMutex.Unlock()

    // Infinite loop to handle WebSocket messages
    for {
        // Read message from agent
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		if messageType != websocket.TextMessage {
			return
		}

		var responseData ResponseData
		err = json.Unmarshal(message, &responseData)
		if err != nil {
			log.Println("Failed to decode responseData JSON")
			return
		}

		key := agentID + ":" + responseData.ResponseID
        responseMutex.Lock()
        responses[key] = responseData.Payload
        responseMutex.Unlock()
    }

    // Lock mutex before accessing agents map
    agentMutex.Lock()
    delete(agents, agentID) // Remove agent from map when connection closes
    agentMutex.Unlock()
}

func tunnelHandler(w http.ResponseWriter, r *http.Request) {
    agentID := r.Header.Get("AGENT-ID")
	if agentID == "" {
        http.Error(w, "AGENT-ID not provided", http.StatusBadRequest)
        return
    }

	var requestData RequestData
	err := json.NewDecoder(r.Body).Decode(&requestData)
    if err != nil {
        http.Error(w, "Failed to decode JSON body", http.StatusBadRequest)
        return
    }

	requestID := uuid.NewString()
	message := Message{requestID, agentID, requestData}

    // Retrieve the WebSocket connection for the agentID
    agentMutex.Lock()
    conn, ok := agents[agentID]
    agentMutex.Unlock()

    if !ok {
        http.Error(w, "agentID not found", http.StatusNotFound)
        return
    }

	err = conn.WriteJSON(message)
    if err != nil {
        log.Println("Error sending message to agent:", err)
        http.Error(w, "Failed to send message to agent", http.StatusInternalServerError)
        return
    }

	result := ""
	key := agentID + ":" + requestID
	for i := 0; i < 100; i++ { //30 seconds timeout
		time.Sleep(300 * time.Millisecond)
		if val, ok := responses[key]; ok {
            result = val
            responseMutex.Lock()
            delete(responses, key)
            responseMutex.Unlock()
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

    r := mux.NewRouter()

    // WebSocket handler
    r.HandleFunc("/ws", websocketHandler)

    // HTTP GET endpoint for updating agents
    r.HandleFunc("/gateway", tunnelHandler).Methods("POST")

    // Set up HTTP server with the router
    http.Handle("/", r)

    log.Println("Server starting on localhost:8080")
    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
