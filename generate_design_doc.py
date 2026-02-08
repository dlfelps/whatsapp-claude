#!/usr/bin/env python3
"""Generate the system design document in docx format."""

from docx import Document
from docx.shared import Inches, Pt, Cm, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT
from docx.enum.style import WD_STYLE_TYPE


def set_cell_shading(cell, color):
    """Set background shading for a table cell."""
    from docx.oxml.ns import qn
    from lxml import etree
    shading = etree.SubElement(cell._element.get_or_add_tcPr(), qn("w:shd"))
    shading.set(qn("w:fill"), color)
    shading.set(qn("w:val"), "clear")


def add_header_row(table, row_idx, values):
    """Style a header row in a table."""
    row = table.rows[row_idx]
    for i, val in enumerate(values):
        cell = row.cells[i]
        cell.text = ""
        p = cell.paragraphs[0]
        run = p.add_run(val)
        run.bold = True
        run.font.size = Pt(9)
        run.font.color.rgb = RGBColor(0xFF, 0xFF, 0xFF)
        p.alignment = WD_ALIGN_PARAGRAPH.LEFT
        set_cell_shading(cell, "4472C4")


def add_data_row(table, row_idx, values, mono_cols=None):
    """Add data to a table row."""
    mono_cols = mono_cols or []
    row = table.rows[row_idx]
    for i, val in enumerate(values):
        cell = row.cells[i]
        cell.text = ""
        p = cell.paragraphs[0]
        run = p.add_run(val)
        run.font.size = Pt(9)
        if i in mono_cols:
            run.font.name = "Courier New"
        if row_idx % 2 == 0:
            set_cell_shading(cell, "D9E2F3")


def add_code_block(doc, text):
    """Add a code-style block to the document."""
    p = doc.add_paragraph()
    p.paragraph_format.left_indent = Cm(1)
    p.paragraph_format.space_before = Pt(4)
    p.paragraph_format.space_after = Pt(4)
    run = p.add_run(text)
    run.font.name = "Courier New"
    run.font.size = Pt(8.5)
    run.font.color.rgb = RGBColor(0x33, 0x33, 0x33)


def build_document():
    doc = Document()

    # -- Document styles --
    style = doc.styles["Normal"]
    style.font.name = "Calibri"
    style.font.size = Pt(11)
    style.paragraph_format.space_after = Pt(6)

    for level in range(1, 4):
        h = doc.styles[f"Heading {level}"]
        h.font.color.rgb = RGBColor(0x1F, 0x38, 0x64)

    # ========================================================================
    # TITLE PAGE
    # ========================================================================
    for _ in range(6):
        doc.add_paragraph()

    title = doc.add_paragraph()
    title.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = title.add_run("System Design Document")
    run.bold = True
    run.font.size = Pt(28)
    run.font.color.rgb = RGBColor(0x1F, 0x38, 0x64)

    subtitle = doc.add_paragraph()
    subtitle.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = subtitle.add_run("WhatsApp-Style Real-Time Messaging Server")
    run.font.size = Pt(16)
    run.font.color.rgb = RGBColor(0x44, 0x72, 0xC4)

    doc.add_paragraph()

    meta = doc.add_paragraph()
    meta.alignment = WD_ALIGN_PARAGRAPH.CENTER
    run = meta.add_run("Version 1.0\nFebruary 2026")
    run.font.size = Pt(12)
    run.font.color.rgb = RGBColor(0x66, 0x66, 0x66)

    doc.add_page_break()

    # ========================================================================
    # TABLE OF CONTENTS (placeholder)
    # ========================================================================
    doc.add_heading("Table of Contents", level=1)
    toc_items = [
        "1. Introduction",
        "2. System Overview",
        "3. Architecture",
        "4. Entity Models",
        "    4.1 User",
        "    4.2 Chat",
        "    4.3 Message",
        "    4.4 Client",
        "    4.5 Attachment",
        "    4.6 InboxEntry",
        "    4.7 Entity Relationship Summary",
        "5. Protocol & API Surface",
        "    5.1 HTTP REST Endpoints",
        "    5.2 WebSocket Endpoint",
        "    5.3 WebSocket Commands (Client to Server)",
        "    5.4 WebSocket Events (Server to Client)",
        "6. Data Flow",
        "    6.1 Connection Lifecycle",
        "    6.2 Chat Creation Flow",
        "    6.3 Message Sending & Delivery",
        "    6.4 Offline Message Queue (Inbox)",
        "    6.5 Message Acknowledgement",
        "    6.6 Participant Management",
        "    6.7 File Attachment Flow",
        "    6.8 TTL Expiration & Cleanup",
        "7. Concurrency & Thread Safety",
        "8. Configuration Reference",
        "9. Project Structure",
    ]
    for item in toc_items:
        p = doc.add_paragraph(item)
        p.paragraph_format.space_before = Pt(1)
        p.paragraph_format.space_after = Pt(1)

    doc.add_page_break()

    # ========================================================================
    # 1. INTRODUCTION
    # ========================================================================
    doc.add_heading("1. Introduction", level=1)
    doc.add_paragraph(
        "This document describes the architecture, entity models, and data flow "
        "of a WhatsApp-style real-time messaging server. The system provides "
        "instant messaging capabilities including chat rooms, file attachments, "
        "offline message queuing, and message acknowledgement, all delivered "
        "over WebSocket connections with a complementary HTTP API for file transfers."
    )
    doc.add_paragraph(
        "The server is implemented in Go (1.21+) using the standard library's "
        "net/http package for HTTP handling and the gorilla/websocket library for "
        "WebSocket communication. All data is stored in-memory with thread-safe "
        "concurrent access."
    )

    # ========================================================================
    # 2. SYSTEM OVERVIEW
    # ========================================================================
    doc.add_heading("2. System Overview", level=1)

    doc.add_heading("Purpose", level=3)
    doc.add_paragraph(
        "The system is a real-time messaging server that enables multiple clients to:"
    )
    bullets = [
        "Connect via WebSocket and receive a unique user identity.",
        "Create chat rooms with up to 100 participants.",
        "Send and receive messages in real time.",
        "Upload and download file attachments (images, video, audio, documents).",
        "Receive queued messages that arrived while the user was offline.",
        "Acknowledge message receipt to clean up the offline queue.",
    ]
    for b in bullets:
        doc.add_paragraph(b, style="List Bullet")

    doc.add_heading("Technology Stack", level=3)
    t = doc.add_table(rows=6, cols=2)
    t.alignment = WD_TABLE_ALIGNMENT.LEFT
    add_header_row(t, 0, ["Component", "Technology"])
    add_data_row(t, 1, ["Language", "Go 1.21+"])
    add_data_row(t, 2, ["HTTP Server", "Go standard library net/http"])
    add_data_row(t, 3, ["WebSocket", "gorilla/websocket v1.5.1"])
    add_data_row(t, 4, ["UUID Generation", "google/uuid v1.4.0"])
    add_data_row(t, 5, ["Data Storage", "In-memory (sync.RWMutex-protected maps)"])

    # ========================================================================
    # 3. ARCHITECTURE
    # ========================================================================
    doc.add_heading("3. Architecture", level=1)

    doc.add_heading("Layered Architecture", level=2)
    doc.add_paragraph(
        "The system follows a clean three-tier layered architecture with strict "
        "dependency direction flowing downward. Each layer has a well-defined "
        "responsibility, and dependencies are injected via constructors."
    )

    # Architecture diagram as text
    add_code_block(doc,
        "+-------------------------------------------------+\n"
        "|           HTTP / WebSocket Handlers              |\n"
        "|   (chat/handler.go, attachment/handler.go)       |\n"
        "|   Request parsing, response encoding, routing    |\n"
        "+-------------------------------------------------+\n"
        "                      |\n"
        "                      v\n"
        "+-------------------------------------------------+\n"
        "|            Business Logic Services               |\n"
        "|   (chat/service.go, attachment/service.go)       |\n"
        "|   Validation, business rules, orchestration      |\n"
        "+-------------------------------------------------+\n"
        "                      |\n"
        "                      v\n"
        "+-------------------------------------------------+\n"
        "|              Data Access Store                   |\n"
        "|   (store/store.go)                               |\n"
        "|   Thread-safe CRUD, in-memory persistence        |\n"
        "+-------------------------------------------------+"
    )

    doc.add_heading("Dependency Injection", level=2)
    doc.add_paragraph(
        "All dependencies are passed explicitly through constructors. There are "
        "no global singletons. The main.go file wires dependencies at startup:"
    )
    add_code_block(doc,
        "s := store.NewStore()\n"
        "chatService := chat.NewService(s)\n"
        "chatHandler := chat.NewHandler(s, chatService)\n"
        "attachService := attachment.NewService(s)\n"
        "attachHandler := attachment.NewHandler(attachService)"
    )

    doc.add_heading("Concurrency Model", level=2)
    doc.add_paragraph(
        "The server manages multiple goroutines per connection and background workers:"
    )
    goroutines = [
        "One goroutine per connection for the read loop (handleConnection).",
        "One goroutine per connection for ping/pong keep-alive (pingLoop).",
        "One background goroutine for TTL-based inbox cleanup (TTLPurger).",
        "The main server goroutine for accepting new connections.",
    ]
    for g in goroutines:
        doc.add_paragraph(g, style="List Bullet")

    # ========================================================================
    # 4. ENTITY MODELS
    # ========================================================================
    doc.add_heading("4. Entity Models", level=1)
    doc.add_paragraph(
        "All domain entities are defined in model/entities.go. The following "
        "sections describe each entity, its fields, and its role in the system."
    )

    # -- 4.1 User --
    doc.add_heading("4.1 User", level=2)
    doc.add_paragraph(
        "Represents a registered user in the system. Users are created "
        "automatically when a WebSocket client connects."
    )
    t = doc.add_table(rows=4, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ID", "string", "id", "Unique identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["Name", "string", "name", "Display name"], [0, 1, 2])
    add_data_row(t, 3, ["Email", "string", "email", "Email address"], [0, 1, 2])

    # -- 4.2 Chat --
    doc.add_heading("4.2 Chat", level=2)
    doc.add_paragraph(
        "Represents a chat room or group conversation. A chat can have between "
        "1 and 100 participants. The creator is always included as a participant."
    )
    t = doc.add_table(rows=5, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ID", "string", "id", "Unique identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["Name", "string", "name", "Chat room display name"], [0, 1, 2])
    add_data_row(t, 3, ["Participants", "[]string", "participants", "List of user IDs in this chat"], [0, 1, 2])
    add_data_row(t, 4, ["CreatedAt", "time.Time", "createdAt", "Timestamp of chat creation"], [0, 1, 2])

    doc.add_paragraph(
        "Constraints: Maximum 100 participants per chat (enforced by "
        "chat.ErrMaxParticipants). The creator's user ID is always "
        "prepended to the participant list."
    )

    # -- 4.3 Message --
    doc.add_heading("4.3 Message", level=2)
    doc.add_paragraph(
        "Represents a message sent within a chat. Messages may optionally "
        "include file attachment references."
    )
    t = doc.add_table(rows=7, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ID", "string", "id", "Unique identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["ChatID", "string", "chatId", "ID of the parent chat"], [0, 1, 2])
    add_data_row(t, 3, ["SenderID", "string", "senderId", "User ID of the sender"], [0, 1, 2])
    add_data_row(t, 4, ["Body", "string", "body", "Text content of the message"], [0, 1, 2])
    add_data_row(t, 5, ["Attachments", "[]string", "attachments", "List of attachment IDs (optional)"], [0, 1, 2])
    add_data_row(t, 6, ["Timestamp", "time.Time", "timestamp", "When the message was sent"], [0, 1, 2])

    # -- 4.4 Client --
    doc.add_heading("4.4 Client", level=2)
    doc.add_paragraph(
        "Represents an active WebSocket connection. Each connected user has "
        "exactly one Client instance. If a user reconnects, the old Client "
        "is replaced."
    )
    t = doc.add_table(rows=5, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ClientID", "string", "-", "Unique connection identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["UserID", "string", "-", "Associated user identifier"], [0, 1, 2])
    add_data_row(t, 3, ["Conn", "*websocket.Conn", "-", "Active WebSocket connection"], [0, 1, 2])
    add_data_row(t, 4, ["WriteMu", "sync.Mutex", "-", "Mutex serializing writes to Conn"], [0, 1, 2])

    doc.add_paragraph(
        "The Client entity provides thread-safe WriteJSON() and WriteMessage() "
        "methods that acquire WriteMu before writing to the WebSocket connection, "
        "preventing message interleaving from concurrent goroutines."
    )

    # -- 4.5 Attachment --
    doc.add_heading("4.5 Attachment", level=2)
    doc.add_paragraph(
        "Represents an uploaded file. Attachments are uploaded via the HTTP REST "
        "API and referenced by ID in messages."
    )
    t = doc.add_table(rows=6, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ID", "string", "id", "Unique identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["Data", "[]byte", "-", "Raw file bytes (not serialized to JSON)"], [0, 1, 2])
    add_data_row(t, 3, ["ContentType", "string", "contentType", "MIME type (e.g. image/png)"], [0, 1, 2])
    add_data_row(t, 4, ["Filename", "string", "filename", "Original file name"], [0, 1, 2])
    add_data_row(t, 5, ["Size", "int64", "size", "File size in bytes"], [0, 1, 2])

    doc.add_paragraph(
        "Constraints: Maximum file size is 10 MB. Only whitelisted MIME types "
        "are accepted: image/jpeg, image/png, image/gif, image/webp, "
        "video/mp4, video/webm, audio/mpeg, audio/ogg, audio/wav, "
        "application/pdf, text/plain, and application/octet-stream."
    )

    # -- 4.6 InboxEntry --
    doc.add_heading("4.6 InboxEntry", level=2)
    doc.add_paragraph(
        "Represents a message queued for an offline user. When a message is "
        "sent to a user who is not currently connected, an InboxEntry is "
        "created. It is delivered when the user reconnects and removed when "
        "the user acknowledges receipt."
    )
    t = doc.add_table(rows=5, cols=4)
    add_header_row(t, 0, ["Field", "Type", "JSON Key", "Description"])
    add_data_row(t, 1, ["ID", "string", "id", "Unique entry identifier (UUID)"], [0, 1, 2])
    add_data_row(t, 2, ["UserID", "string", "userId", "Target user for delivery"], [0, 1, 2])
    add_data_row(t, 3, ["Message", "*Message", "message", "Pointer to the queued message"], [0, 1, 2])
    add_data_row(t, 4, ["CreatedAt", "time.Time", "createdAt", "When the entry was queued"], [0, 1, 2])

    doc.add_paragraph(
        "InboxEntries have a TTL of 30 days. A background TTLPurger goroutine "
        "runs every 60 seconds to remove expired entries."
    )

    # -- 4.7 Entity Relationship Summary --
    doc.add_heading("4.7 Entity Relationship Summary", level=2)

    add_code_block(doc,
        "+--------+       +--------+       +-----------+\n"
        "|  User  |1----*|  Chat  |1----*| Message   |\n"
        "|--------|       |--------|       |-----------|\n"
        "| ID     |       | ID     |       | ID        |\n"
        "| Name   |       | Name   |       | ChatID    |\n"
        "| Email  |       | Partic.|       | SenderID  |\n"
        "+--------+       +--------+       | Body      |\n"
        "    |                              | Attach[]  |\n"
        "    |                              | Timestamp |\n"
        "    |                              +-----------+\n"
        "    |                                    |\n"
        "    |1                                   |*\n"
        "    |         +------------+       +------------+\n"
        "    +-------*| InboxEntry |       | Attachment |\n"
        "    |         |------------|       |------------|\n"
        "    |         | ID         |       | ID         |\n"
        "    |         | UserID     |       | Data       |\n"
        "    |         | Message*   |       | ContentType|\n"
        "    |         | CreatedAt  |       | Filename   |\n"
        "    |         +------------+       | Size       |\n"
        "    |                              +------------+\n"
        "    |1\n"
        "    |\n"
        "+--------+\n"
        "| Client |\n"
        "|--------|\n"
        "| ClientID|\n"
        "| UserID  |\n"
        "| Conn    |\n"
        "| WriteMu |\n"
        "+--------+"
    )

    relationships = [
        "A User can participate in many Chats (many-to-many via Participants list).",
        "A Chat contains many Messages (one-to-many via ChatID).",
        "A Message is sent by one User (many-to-one via SenderID).",
        "A Message references zero or more Attachments (many-to-many via Attachments list).",
        "A User has zero or more InboxEntries (one-to-many via UserID).",
        "An InboxEntry references exactly one Message (many-to-one via Message pointer).",
        "A User has zero or one active Client (one-to-one via UserID in the clients map).",
    ]
    for r in relationships:
        doc.add_paragraph(r, style="List Bullet")

    # ========================================================================
    # 5. PROTOCOL & API SURFACE
    # ========================================================================
    doc.add_heading("5. Protocol & API Surface", level=1)

    # 5.1 HTTP REST
    doc.add_heading("5.1 HTTP REST Endpoints", level=2)
    t = doc.add_table(rows=3, cols=4)
    add_header_row(t, 0, ["Method", "Path", "Purpose", "Handler"])
    add_data_row(t, 1, ["POST", "/attachments", "Upload a file attachment", "attachment.Handler.HandleUpload()"], [1, 3])
    add_data_row(t, 2, ["GET", "/attachments/{id}", "Download an attachment by ID", "attachment.Handler.HandleDownload()"], [1, 3])

    doc.add_paragraph(
        "File uploads use multipart/form-data encoding with the file in a field "
        "named \"file\". Successful uploads return HTTP 201 with a JSON body "
        "containing the attachment metadata (ID, filename, size, contentType)."
    )

    # 5.2 WebSocket
    doc.add_heading("5.2 WebSocket Endpoint", level=2)
    t = doc.add_table(rows=2, cols=4)
    add_header_row(t, 0, ["Method", "Path", "Purpose", "Handler"])
    add_data_row(t, 1, ["GET (upgrade)", "/ws", "Upgrade to WebSocket", "chat.Handler.HandleWebSocket()"], [1, 3])

    doc.add_paragraph(
        "Upon successful connection, the server sends a \"connected\" event "
        "containing the assigned userID and clientID, followed by any queued "
        "inbox messages."
    )

    # 5.3 WebSocket Commands
    doc.add_heading("5.3 WebSocket Commands (Client to Server)", level=2)
    doc.add_paragraph(
        "Clients send commands as JSON objects with a \"type\" field and a "
        "\"payload\" field containing command-specific data."
    )

    add_code_block(doc,
        'WSCommand {\n'
        '    "type":    string,          // Command type identifier\n'
        '    "payload": json.RawMessage  // Command-specific JSON payload\n'
        '}'
    )

    t = doc.add_table(rows=5, cols=3)
    add_header_row(t, 0, ["Command Type", "Payload Fields", "Description"])
    add_data_row(t, 1, ["createChat", "name, participants[]", "Create a new chat room"], [0])
    add_data_row(t, 2, ["sendMessage", "chatId, body, attachments[]", "Send a message to a chat"], [0])
    add_data_row(t, 3, ["ack", "messageId", "Acknowledge receipt of a message"], [0])
    add_data_row(t, 4, ["modifyParticipants", 'chatId, action ("add"/"remove"), userIds[]', "Add or remove chat participants"], [0])

    # 5.4 WebSocket Events
    doc.add_heading("5.4 WebSocket Events (Server to Client)", level=2)
    doc.add_paragraph(
        "The server pushes events to clients as JSON objects with a \"type\" "
        "field and a \"payload\" field."
    )

    add_code_block(doc,
        'WSEvent {\n'
        '    "type":    string,      // Event type identifier\n'
        '    "payload": interface{}  // Event-specific data\n'
        '}'
    )

    t = doc.add_table(rows=6, cols=3)
    add_header_row(t, 0, ["Event Type", "Payload", "When Sent"])
    add_data_row(t, 1, ["connected", "userID, clientID", "Client successfully connects"], [0])
    add_data_row(t, 2, ["newMessage", "Message object", "New message arrives or delivered from inbox"], [0])
    add_data_row(t, 3, ["chatUpdate", "Chat object", "Chat created or participants modified"], [0])
    add_data_row(t, 4, ["messageAcked", "messageID, ackedBy", "Message acknowledged by recipient"], [0])
    add_data_row(t, 5, ["error", "message string", "An operation failed"], [0])

    # ========================================================================
    # 6. DATA FLOW
    # ========================================================================
    doc.add_heading("6. Data Flow", level=1)

    # 6.1 Connection Lifecycle
    doc.add_heading("6.1 Connection Lifecycle", level=2)
    doc.add_paragraph(
        "The following describes the full lifecycle of a WebSocket connection "
        "from initial handshake to disconnection."
    )

    add_code_block(doc,
        "Client                          Server\n"
        "  |                               |\n"
        "  |--- HTTP GET /ws ------------->|\n"
        "  |                               |  Upgrade HTTP to WebSocket\n"
        "  |<-- 101 Switching Protocols ---|  Generate userID, clientID\n"
        "  |                               |  Register client in Store\n"
        "  |<-- connected event -----------|  {userID, clientID}\n"
        "  |<-- newMessage events ---------|  Deliver queued inbox messages\n"
        "  |                               |\n"
        "  |--- commands ----------------->|  Read loop (handleConnection)\n"
        "  |<-- events -------------------|  Write via WriteJSON\n"
        "  |                               |\n"
        "  |<-- ping ---------------------|  Every 50 seconds\n"
        "  |--- pong -------------------->|  Must respond within 60s\n"
        "  |                               |\n"
        "  |--- close / disconnect ------->|  Unregister client from Store\n"
        "  |                               |  Future messages go to inbox"
    )

    steps = [
        "The client initiates an HTTP GET request to /ws.",
        "The server upgrades the connection to WebSocket using gorilla/websocket's Upgrader.",
        "A new User and Client are created with unique UUIDs.",
        "The client is registered in the Store's clients map (keyed by userID).",
        "A \"connected\" event is sent to the client with its assigned userID and clientID.",
        "Any queued InboxEntries for this user are delivered as \"newMessage\" events.",
        "A read loop goroutine (handleConnection) and a ping loop goroutine (pingLoop) are started.",
        "The read loop processes incoming commands until the connection closes.",
        "On disconnect, the client is unregistered. Messages sent after this go to the inbox.",
    ]
    for i, s in enumerate(steps, 1):
        doc.add_paragraph(f"{i}. {s}")

    # 6.2 Chat Creation
    doc.add_heading("6.2 Chat Creation Flow", level=2)

    add_code_block(doc,
        "Client                   Handler                  Service                  Store\n"
        "  |                         |                        |                       |\n"
        "  |-- createChat command -->|                        |                       |\n"
        "  |                         |-- CreateChat() ------->|                       |\n"
        "  |                         |                        |  Validate participant  |\n"
        "  |                         |                        |  count (max 100)       |\n"
        "  |                         |                        |  Prepend creator ID    |\n"
        "  |                         |                        |-- CreateChat() ------->|\n"
        "  |                         |                        |                  Save to map\n"
        "  |                         |                        |<-- Chat object --------|\n"
        "  |                         |<-- Chat object --------|                       |\n"
        "  |                         |                        |                       |\n"
        "  |                         |-- broadcastToParticipants() -------+           |\n"
        "  |<-- chatUpdate event ----|<----------------------------------+           |"
    )

    # 6.3 Message Sending & Delivery
    doc.add_heading("6.3 Message Sending & Delivery", level=2)
    doc.add_paragraph(
        "This is the core data flow. Messages are routed differently depending "
        "on whether the recipient is online or offline."
    )

    add_code_block(doc,
        "Sender                   Handler                  Service                  Store\n"
        "  |                         |                        |                       |\n"
        "  |-- sendMessage cmd ----->|                        |                       |\n"
        "  |                         |-- SendMessage() ------>|                       |\n"
        "  |                         |                        |  Validate sender is   |\n"
        "  |                         |                        |  a participant        |\n"
        "  |                         |                        |-- CreateMessage() --->|\n"
        "  |                         |                        |                  Save to map\n"
        "  |                         |<-- Message object -----|                       |\n"
        "  |                         |                        |                       |\n"
        "  |                         |-- deliverMessage() --->|                       |\n"
        "  |                         |   For each participant:|                       |\n"
        "  |                         |   +-- Is online? ------|---GetClient()-------->|\n"
        "  |                         |   |   YES: send        |                       |\n"
        "  |                         |   |   newMessage event  |                       |\n"
        "  |                         |   |   NO: queue in     |---AddToInbox()------->|\n"
        "  |                         |   |   inbox             |                       |\n"
        "  |                         |   +--------------------+                       |"
    )

    doc.add_paragraph(
        "Key behavior: The sender does NOT receive their own message back. "
        "The system skips the sender when iterating over participants."
    )

    # 6.4 Offline Queue
    doc.add_heading("6.4 Offline Message Queue (Inbox)", level=2)
    doc.add_paragraph(
        "The inbox system ensures message delivery reliability for disconnected users."
    )
    steps = [
        "When a message targets an offline user, an InboxEntry is created with a unique ID, "
        "the target userID, a pointer to the Message, and the current timestamp.",
        "InboxEntries are stored in the Store's inbox map, keyed by userID.",
        "When the user reconnects, all their InboxEntries are retrieved and delivered "
        "as \"newMessage\" events.",
        "The user acknowledges each message with an \"ack\" command.",
        "Upon acknowledgement, the InboxEntry is removed from the inbox map.",
        "Unacknowledged entries expire after 30 days and are purged by the TTLPurger.",
    ]
    for i, s in enumerate(steps, 1):
        doc.add_paragraph(f"{i}. {s}")

    # 6.5 Message Acknowledgement
    doc.add_heading("6.5 Message Acknowledgement", level=2)

    add_code_block(doc,
        "Recipient                Handler                  Service                  Store\n"
        "  |                         |                        |                       |\n"
        "  |-- ack command --------->|                        |                       |\n"
        "  |  {messageId}            |-- AcknowledgeMessage-->|                       |\n"
        "  |                         |                        |-- GetMessage() ------>|\n"
        "  |                         |                        |<- Message object -----|  \n"
        "  |                         |                        |-- RemoveInboxEntry()->|\n"
        "  |                         |                        |          Remove by msgID\n"
        "  |                         |<-- senderID, msgID ----|                       |\n"
        "  |                         |                        |                       |\n"
        "  |                         |-- Send messageAcked -->|                       |\n"
        "  |                         |   event to sender      |                       |"
    )

    doc.add_paragraph(
        "The acknowledgement flow notifies the original sender that the "
        "recipient has received the message. The InboxEntry is deleted from "
        "the store upon successful acknowledgement."
    )

    # 6.6 Participant Management
    doc.add_heading("6.6 Participant Management", level=2)
    doc.add_paragraph(
        "Participants can be added to or removed from a chat using the "
        "modifyParticipants command. The action field must be \"add\" or \"remove\"."
    )
    steps = [
        "The client sends a modifyParticipants command with chatId, action, and userIds.",
        "The service validates that the requesting user is a current participant.",
        "For \"add\": new user IDs are appended (duplicates checked, max 100 enforced).",
        "For \"remove\": specified user IDs are removed from the participant list.",
        "The updated Chat object is broadcast as a \"chatUpdate\" event to all "
        "current participants (including newly added ones).",
    ]
    for i, s in enumerate(steps, 1):
        doc.add_paragraph(f"{i}. {s}")

    # 6.7 File Attachment
    doc.add_heading("6.7 File Attachment Flow", level=2)

    doc.add_heading("Upload", level=3)
    add_code_block(doc,
        "Client                   Handler                  Service                  Store\n"
        "  |                         |                        |                       |\n"
        "  |-- POST /attachments --->|                        |                       |\n"
        "  |   multipart/form-data   |-- Upload() ---------->|                       |\n"
        "  |                         |                        |  Validate size <= 10MB|\n"
        "  |                         |                        |  Validate MIME type   |\n"
        "  |                         |                        |-- CreateAttachment()->|\n"
        "  |                         |                        |              Save to map\n"
        "  |                         |<-- Attachment object --|                       |\n"
        "  |<-- 201 {id, filename,   |                        |                       |\n"
        "  |    size, contentType} --|                        |                       |"
    )

    doc.add_heading("Download", level=3)
    add_code_block(doc,
        "Client                   Handler                  Service                  Store\n"
        "  |                         |                        |                       |\n"
        "  |-- GET /attachments/id ->|                        |                       |\n"
        "  |                         |-- Get() -------------->|                       |\n"
        "  |                         |                        |-- GetAttachment() --->|\n"
        "  |                         |                        |<-- Attachment --------|\n"
        "  |                         |<-- Attachment ---------|                       |\n"
        "  |<-- 200 (raw file data)  |  Set Content-Type      |                       |\n"
        "  |   Content-Disposition   |  Set Content-Disposition|                      |"
    )

    doc.add_paragraph(
        "Attachments are referenced in messages by their ID. The client first uploads "
        "the file via the REST API, receives the attachment ID, then includes that ID "
        "in the attachments field of a sendMessage command."
    )

    # 6.8 TTL Expiration
    doc.add_heading("6.8 TTL Expiration & Cleanup", level=2)

    add_code_block(doc,
        "TTLPurger (background goroutine)\n"
        "  |\n"
        "  |-- Every 60 seconds (DefaultPurgeInterval) ------+\n"
        "  |                                                  |\n"
        "  +-- Store.PurgeExpiredInboxEntries(30 days) -------+\n"
        "      |                                              |\n"
        "      |  For each userID in inbox map:               |\n"
        "      |    For each InboxEntry:                      |\n"
        "      |      If CreatedAt + 30 days < now:           |\n"
        "      |        Remove entry                          |\n"
        "      |    If user's inbox is empty:                 |\n"
        "      |      Delete user key from map                |"
    )

    doc.add_paragraph(
        "The TTLPurger is started in main.go and controlled via a context.Context. "
        "It uses a time.Ticker to run at regular intervals. During graceful shutdown, "
        "the context is cancelled, stopping the purger."
    )

    # ========================================================================
    # 7. CONCURRENCY & THREAD SAFETY
    # ========================================================================
    doc.add_heading("7. Concurrency & Thread Safety", level=1)

    doc.add_heading("Store-Level Locking", level=2)
    doc.add_paragraph(
        "The Store uses fine-grained locking with separate sync.RWMutex instances "
        "for each data collection. This allows concurrent reads and minimizes "
        "lock contention."
    )

    t = doc.add_table(rows=7, cols=3)
    add_header_row(t, 0, ["Mutex", "Protects", "Lock Pattern"])
    add_data_row(t, 1, ["usersMu", "users map", "RLock for reads, Lock for writes"], [0])
    add_data_row(t, 2, ["chatsMu", "chats map", "RLock for reads, Lock for writes"], [0])
    add_data_row(t, 3, ["messagesMu", "messages map", "RLock for reads, Lock for writes"], [0])
    add_data_row(t, 4, ["clientsMu", "clients map", "RLock for reads, Lock for writes"], [0])
    add_data_row(t, 5, ["attachmentsMu", "attachments map", "RLock for reads, Lock for writes"], [0])
    add_data_row(t, 6, ["inboxMu", "inbox map", "RLock for reads, Lock for writes"], [0])

    doc.add_heading("Connection-Level Safety", level=2)
    safety_points = [
        "WebSocket Write Mutex: Each Client has a WriteMu (sync.Mutex) that serializes "
        "all writes to the WebSocket connection, preventing message interleaving.",
        "Client Replacement Guard: When unregistering a client, the Store compares "
        "the ClientID to ensure a stale goroutine does not accidentally remove a "
        "newer connection for the same user.",
        "Graceful Shutdown: The server uses context.WithCancel() to coordinate "
        "shutdown across all goroutines. A 30-second timeout prevents indefinite hanging.",
    ]
    for s in safety_points:
        doc.add_paragraph(s, style="List Bullet")

    # ========================================================================
    # 8. CONFIGURATION REFERENCE
    # ========================================================================
    doc.add_heading("8. Configuration Reference", level=1)

    doc.add_heading("Server Configuration", level=2)
    t = doc.add_table(rows=5, cols=3)
    add_header_row(t, 0, ["Parameter", "Value", "Location"])
    add_data_row(t, 1, ["Listen Address", ":8080", "main.go"], [0, 2])
    add_data_row(t, 2, ["Read Timeout", "15 seconds", "main.go"], [0, 2])
    add_data_row(t, 3, ["Write Timeout", "15 seconds", "main.go"], [0, 2])
    add_data_row(t, 4, ["Idle Timeout", "60 seconds", "main.go"], [0, 2])

    doc.add_heading("WebSocket Configuration", level=2)
    t = doc.add_table(rows=5, cols=3)
    add_header_row(t, 0, ["Parameter", "Value", "Location"])
    add_data_row(t, 1, ["Write Wait", "10 seconds", "chat/handler.go"], [0, 2])
    add_data_row(t, 2, ["Pong Wait", "60 seconds", "chat/handler.go"], [0, 2])
    add_data_row(t, 3, ["Ping Period", "50 seconds", "chat/handler.go"], [0, 2])
    add_data_row(t, 4, ["Max Message Size", "512 KB", "chat/handler.go"], [0, 2])

    doc.add_heading("Attachment Configuration", level=2)
    t = doc.add_table(rows=2, cols=3)
    add_header_row(t, 0, ["Parameter", "Value", "Location"])
    add_data_row(t, 1, ["Max File Size", "10 MB", "attachment/service.go"], [0, 2])

    doc.add_heading("TTL Configuration", level=2)
    t = doc.add_table(rows=3, cols=3)
    add_header_row(t, 0, ["Parameter", "Value", "Location"])
    add_data_row(t, 1, ["Inbox Entry TTL", "30 days", "store/ttl.go"], [0, 2])
    add_data_row(t, 2, ["Purge Interval", "60 seconds", "store/ttl.go"], [0, 2])

    # ========================================================================
    # 9. PROJECT STRUCTURE
    # ========================================================================
    doc.add_heading("9. Project Structure", level=1)

    add_code_block(doc,
        "whatsapp-claude/\n"
        "+-- main.go                    Server entry point, DI wiring, graceful shutdown\n"
        "+-- go.mod / go.sum            Go module definition and dependency checksums\n"
        "+-- model/\n"
        "|   +-- entities.go            All domain entities and protocol types\n"
        "+-- store/\n"
        "|   +-- store.go               Thread-safe in-memory data store (CRUD)\n"
        "|   +-- ttl.go                 Background TTL purger for expired inbox entries\n"
        "|   +-- store_test.go          Store unit tests\n"
        "+-- chat/\n"
        "|   +-- service.go             Chat business logic (create, send, modify, ack)\n"
        "|   +-- handler.go             WebSocket handler (connect, dispatch, deliver)\n"
        "|   +-- service_test.go        Chat service unit tests\n"
        "+-- attachment/\n"
        "|   +-- service.go             File upload validation and retrieval\n"
        "|   +-- handler.go             HTTP handlers for upload and download\n"
        "|   +-- service_test.go        Attachment service unit tests\n"
        "+-- integration/\n"
        "|   +-- websocket_test.go      End-to-end WebSocket integration tests\n"
        "|   +-- attachment_test.go     End-to-end HTTP attachment tests\n"
        "+-- cmd/\n"
        "    +-- testclient/\n"
        "        +-- main.go            Interactive command-line test client"
    )

    t = doc.add_table(rows=8, cols=3)
    add_header_row(t, 0, ["Package", "Responsibility", "Key Files"])
    add_data_row(t, 1, ["main", "Server bootstrap, dependency injection, shutdown", "main.go"], [0, 2])
    add_data_row(t, 2, ["model", "Domain entities and protocol message types", "entities.go"], [0, 2])
    add_data_row(t, 3, ["store", "Thread-safe in-memory persistence layer", "store.go, ttl.go"], [0, 2])
    add_data_row(t, 4, ["chat", "Real-time messaging business logic and WebSocket handling", "service.go, handler.go"], [0, 2])
    add_data_row(t, 5, ["attachment", "File upload/download validation and HTTP handling", "service.go, handler.go"], [0, 2])
    add_data_row(t, 6, ["integration", "End-to-end integration tests", "websocket_test.go, attachment_test.go"], [0, 2])
    add_data_row(t, 7, ["cmd/testclient", "Interactive CLI for manual testing", "main.go"], [0, 2])

    # Save
    doc.save("/home/user/whatsapp-claude/design-document.docx")
    print("Design document saved to design-document.docx")


if __name__ == "__main__":
    build_document()
