# WhatsApp-Style Messaging Server

A real-time messaging server written in Go that provides WhatsApp-like functionality with WebSocket communication.

## Features

- **Real-time messaging** via WebSocket connections
- **Chat rooms** with up to 100 participants
- **File attachments** with automatic TTL-based cleanup
- **Offline message queue** (inbox) for users who reconnect
- **Message acknowledgement** system

## Project Structure

```
├── main.go              # Server entry point
├── chat/                # Chat service and WebSocket handler
├── attachment/          # Attachment upload/download handlers
├── model/               # Domain entities (User, Chat, Message, etc.)
├── store/               # In-memory data store with TTL support
├── integration/         # Integration tests
└── cmd/testclient/      # WebSocket test client
```

## Getting Started

### Prerequisites

- Go 1.21 or later

### Running the Server

```bash
go run main.go
```

The server starts on port 8080.

### API Endpoints

- `GET /ws` - WebSocket connection (requires `?userId=<id>` query parameter)
- `POST /attachments` - Upload attachment
- `GET /attachments/<id>` - Download attachment

### WebSocket Commands

| Command | Description |
|---------|-------------|
| `createChat` | Create a new chat with participants |
| `sendMessage` | Send a message to a chat |
| `modifyParticipants` | Add or remove chat participants |
| `ack` | Acknowledge receipt of a message |

### WebSocket Events

| Event | Description |
|-------|-------------|
| `connected` | Sent on successful connection |
| `newMessage` | New message received |
| `chatCreated` | Chat was created |
| `participantsModified` | Chat participants changed |
| `messageAcked` | Message was acknowledged |
| `error` | Error occurred |

## Running Tests

```bash
go test ./...
```

## Dependencies

- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket implementation
- [google/uuid](https://github.com/google/uuid) - UUID generation
