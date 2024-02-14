package main

import (
    "context"
    "encoding/json"
    "flag"
    "log"
    "io/ioutil"
    "os"
    "os/signal"
    "time"
    "net/http"
    "net/url"

    "github.com/gorilla/websocket"
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

func sendGetRquest(url string, requestID string, conn *websocket.Conn) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    client := &http.Client{}
    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        log.Println("Error creating request:", err)
        return
    }
    req = req.WithContext(ctx)

    // Send the HTTP request
    resp, err := client.Do(req)
    if err != nil {
        // Check if the error is due to a timeout
        if ctx.Err() == context.DeadlineExceeded {
            log.Println("Request timed out")
        } else {
            log.Println("Error sending request:", err)
        }
        return
    }
    defer resp.Body.Close()

    // Read the response body into a byte slice
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Println("Error reading response body:", err)
        return
    }

    err = conn.WriteJSON(ResponseData{requestID, string(bodyBytes)})
    if err != nil {
        log.Println("write:", err)
        return
    }
}

func connectToWebSocket(agentID string) {
    // Connect to WebSocket server
    header := http.Header{}
    header.Set("AGENT-ID", agentID) // Set agentID header
    c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", header)
    if err != nil {
        log.Fatal("dial:", err)
    }
    defer c.Close()

    done := make(chan struct{})

    // Start a goroutine to read messages from the WebSocket connection
    go func() {
        defer close(done)
        for {

            // Read message from client
            messageType, message, err := c.ReadMessage()
            if err != nil {
                log.Println("read:", err)
                return
            }

            if messageType != websocket.TextMessage {
                return
            }

            var m Message
            err = json.Unmarshal(message, &m)
            if err != nil {
                log.Println("Failed to decode Message JSON")
                return
            }

            endpoint, err := url.JoinPath(m.RequestData.ServiceName, m.RequestData.ServiceEndpoint)
            if err != nil {
                log.Println(err)
            }

            if m.RequestData.HTTPMethod == "GET" {
                go sendGetRquest(endpoint, m.RequestID, c)
            }

        }
    }()

    // Handle interrupt signal to gracefully close the connection
    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt)

    for {
        select {
        case <-interrupt:
            log.Println("Interrupt signal received, closing connection...")
            err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
            if err != nil {
                log.Println("write close:", err)
                return
            }
            select {
            case <-done:
            case <-time.After(time.Second):
            }
            return
        }
    }
}

func main() {
    agentID := flag.String("id", "", "Agent ID")
    flag.Parse()

    if *agentID == "" {
        log.Fatal("Agent ID not provided")
    }

    log.Println("Connecting to WebSocket server with agentID:", *agentID)
    connectToWebSocket(*agentID)
}
