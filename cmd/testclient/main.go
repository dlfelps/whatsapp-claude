package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/websocket"
)

type WSCommand struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	serverURL := "ws://localhost:8080/ws"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	fmt.Printf("Connecting to %s...\n", serverURL)

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer conn.Close()

	// Handle interrupt
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Read messages in background
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			var event WSEvent
			json.Unmarshal(message, &event)

			// Pretty print
			prettyPayload, _ := json.MarshalIndent(json.RawMessage(event.Payload), "  ", "  ")
			fmt.Printf("\n📨 [%s]\n  %s\n> ", event.Type, string(prettyPayload))
		}
	}()

	fmt.Println("\nCommands:")
	fmt.Println("  chat <name> <user1,user2,...>  - Create a chat")
	fmt.Println("  msg <chatId> <message>         - Send a message")
	fmt.Println("  ack <messageId>                - Acknowledge a message")
	fmt.Println("  add <chatId> <user1,user2,...> - Add participants")
	fmt.Println("  rm <chatId> <user1,user2,...>  - Remove participants")
	fmt.Println("  raw <json>                     - Send raw JSON command")
	fmt.Println("  quit                           - Exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("> ")
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		cmd := parts[0]

		var wsCmd *WSCommand

		switch cmd {
		case "quit", "exit", "q":
			fmt.Println("Goodbye!")
			return

		case "chat":
			if len(parts) < 2 {
				fmt.Println("Usage: chat <name> [user1,user2,...]")
				fmt.Print("> ")
				continue
			}
			name := parts[1]
			var participants []string
			if len(parts) > 2 {
				participants = strings.Split(parts[2], ",")
			}
			wsCmd = &WSCommand{
				Type: "createChat",
				Payload: map[string]interface{}{
					"name":         name,
					"participants": participants,
				},
			}

		case "msg":
			if len(parts) < 3 {
				fmt.Println("Usage: msg <chatId> <message>")
				fmt.Print("> ")
				continue
			}
			wsCmd = &WSCommand{
				Type: "sendMessage",
				Payload: map[string]interface{}{
					"chatId": parts[1],
					"body":   parts[2],
				},
			}

		case "ack":
			if len(parts) < 2 {
				fmt.Println("Usage: ack <messageId>")
				fmt.Print("> ")
				continue
			}
			wsCmd = &WSCommand{
				Type: "ack",
				Payload: map[string]interface{}{
					"messageId": parts[1],
				},
			}

		case "add":
			if len(parts) < 3 {
				fmt.Println("Usage: add <chatId> <user1,user2,...>")
				fmt.Print("> ")
				continue
			}
			wsCmd = &WSCommand{
				Type: "modifyParticipants",
				Payload: map[string]interface{}{
					"chatId":  parts[1],
					"action":  "add",
					"userIds": strings.Split(parts[2], ","),
				},
			}

		case "rm":
			if len(parts) < 3 {
				fmt.Println("Usage: rm <chatId> <user1,user2,...>")
				fmt.Print("> ")
				continue
			}
			wsCmd = &WSCommand{
				Type: "modifyParticipants",
				Payload: map[string]interface{}{
					"chatId":  parts[1],
					"action":  "remove",
					"userIds": strings.Split(parts[2], ","),
				},
			}

		case "raw":
			if len(parts) < 2 {
				fmt.Println("Usage: raw <json>")
				fmt.Print("> ")
				continue
			}
			rawJSON := strings.Join(parts[1:], " ")
			if err := conn.WriteMessage(websocket.TextMessage, []byte(rawJSON)); err != nil {
				fmt.Println("Send error:", err)
			} else {
				fmt.Println("Sent raw JSON")
			}
			fmt.Print("> ")
			continue

		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			fmt.Print("> ")
			continue
		}

		if wsCmd != nil {
			if err := conn.WriteJSON(wsCmd); err != nil {
				fmt.Println("Send error:", err)
			} else {
				fmt.Printf("Sent: %s\n", wsCmd.Type)
			}
		}

		fmt.Print("> ")
	}
}
