// Package main implements an interactive WebSocket test client for manually
// testing the messaging server from the command line.
//
// LEARNING: In Go, the cmd/ directory conventionally holds executable entry
// points. Each subdirectory under cmd/ is a separate binary. This project has:
//   - main.go (root)        → the server binary
//   - cmd/testclient/main.go → the test client binary
//
// To build and run: go run ./cmd/testclient
//
// This file demonstrates:
//   - Command-line argument parsing with os.Args
//   - Using gorilla/websocket as a WebSocket client (not just server)
//   - bufio.Scanner for reading interactive user input line-by-line
//   - Background goroutines for concurrent I/O (reading server events
//     while waiting for user input)
//   - strings.SplitN for limited splitting (preserving spaces in messages)
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

// LEARNING: These types duplicate the server's WSCommand and WSEvent types.
// In a production setup, you'd import them from a shared package. But for
// a standalone test client binary, it's simpler to redefine them here.
//
// Notice the Payload types differ: WSCommand uses interface{} (for sending)
// while WSEvent uses json.RawMessage (for receiving). When sending, we know
// the payload type at compile time. When receiving, we defer parsing until
// we know the event type.
type WSCommand struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	// LEARNING: os.Args is a string slice containing command-line arguments.
	// os.Args[0] is the program name, os.Args[1:] are the user-provided args.
	// This simple pattern provides a default value while allowing override:
	//   go run ./cmd/testclient ws://other-host:9090/ws
	serverURL := "ws://localhost:8080/ws"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	fmt.Printf("Connecting to %s...\n", serverURL)

	// LEARNING: websocket.DefaultDialer.Dial establishes a WebSocket connection
	// to the given URL. The "Dial" name follows Go's networking convention
	// (similar to net.Dial, tls.Dial). The second return value is the HTTP
	// response from the upgrade handshake (usually not needed).
	//
	// log.Fatal logs the error and calls os.Exit(1) — use it when the program
	// cannot continue. It's appropriate here because the client is useless
	// without a server connection.
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer conn.Close()

	// LEARNING: Signal handling allows graceful cleanup on Ctrl+C.
	// signal.Notify directs OS signals to a channel. os.Interrupt maps to
	// SIGINT (Ctrl+C on Unix). The buffered channel (capacity 1) ensures
	// the signal isn't dropped if nothing is listening yet.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// LEARNING: This anonymous goroutine reads server events in the background.
	// The "go func() { ... }()" pattern launches a closure as a goroutine.
	// It runs concurrently with the main goroutine (which handles user input).
	//
	// This is the standard pattern for bidirectional communication:
	//   - Main goroutine: reads stdin, sends commands to server
	//   - Background goroutine: reads from server, prints events to stdout
	//
	// json.MarshalIndent pretty-prints JSON with indentation for readability.
	// The first string arg is the prefix for each line, the second is the indent.
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

	// LEARNING: bufio.NewScanner wraps os.Stdin to read input line-by-line.
	// scanner.Scan() blocks until a line is available, returns true if
	// successful, and false on EOF or error. scanner.Text() returns the
	// line without the trailing newline.
	//
	// This is Go's standard approach for reading interactive console input,
	// similar to Python's input() or Java's Scanner.nextLine().
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("> ")
			continue
		}

		// LEARNING: strings.SplitN splits a string into at most N parts.
		// SplitN(line, " ", 3) splits on the first two spaces, preserving
		// the rest as a single string. This is important for the "msg" command
		// where the message body can contain spaces:
		//   "msg abc-123 Hello World!" → ["msg", "abc-123", "Hello World!"]
		parts := strings.SplitN(line, " ", 3)
		cmd := parts[0]

		var wsCmd *WSCommand

		// LEARNING: Go's switch statement doesn't fall through by default
		// (unlike C/Java). Each case is independent. Multiple values can
		// match a single case: case "quit", "exit", "q":
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
			// LEARNING: map[string]interface{} creates a generic JSON object.
			// When marshaled to JSON, Go maps become JSON objects. This is
			// useful for building dynamic payloads without defining a struct.
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
			// LEARNING: conn.WriteMessage sends raw bytes over the WebSocket.
			// websocket.TextMessage indicates the payload is UTF-8 text (JSON).
			// The other common type is websocket.BinaryMessage for non-text data.
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

		// LEARNING: conn.WriteJSON marshals the struct to JSON and sends it
		// over the WebSocket as a text message. It's equivalent to:
		//   data, _ := json.Marshal(wsCmd)
		//   conn.WriteMessage(websocket.TextMessage, data)
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
