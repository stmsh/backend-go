package client

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	baseClientsCount = 100
	baseRoomsCount   = 10

	writeWait  = 10 * time.Second
	pongWait   = 1 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type MessageOutgoing interface{}

type MessageIncoming struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type Serializer interface {
	Serialize(MessageOutgoing) (int, [][]byte)
}

type Client struct {
	ID     string
	RoomID string

	conn   *websocket.Conn
	egress chan MessageOutgoing

	Serializer Serializer
	Manager    *ConnectionManager
}

func NewClient(
	conn *websocket.Conn,
	manager *ConnectionManager,
	serializer Serializer,
) *Client {
	return &Client{
		ID:         uuid.NewString(),
		RoomID:     "",
		conn:       conn,
		egress:     make(chan MessageOutgoing),
		Serializer: serializer,
		Manager:    manager,
	}
}

func (c *Client) WriteMessages() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.egress:
			if !ok {
				log.Printf("Client %s has disconnected", c.ID)
				return
			}

			messageType, messages := c.Serializer.Serialize(msg)
			for i := range messages {
				c.conn.WriteMessage(messageType, messages[i])
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) ReadMessages() {
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			) {
				log.Printf("error: %v", err)
			}

			c.Manager.RemoveClient(c)
			break
		}

		var msg MessageIncoming
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.ReportError(err)
			continue
		}

		c.Manager.handleMessage(c, msg)
	}
}

func (c *Client) ReportError(err error) {
	c.egress <- err
}

func (c *Client) Send(msg MessageOutgoing) {
	c.egress <- msg
}

type EventHandler func(*Client, MessageIncoming)

type ConnectionManager struct {
	sync.RWMutex
	clients map[string]*Client
	rooms   map[string][]*Client

	// Client will be removed from room before onLeave call.
	// No messages will be delivered to disconnected client.
	onLeave func(c *Client)

	handlers map[string]EventHandler
}

func NewConnectionManager(onLeave func(c *Client)) *ConnectionManager {
	return &ConnectionManager{
		clients: make(map[string]*Client, baseClientsCount),
		rooms:   make(map[string][]*Client, baseRoomsCount),
		onLeave: onLeave,

		handlers: make(map[string]EventHandler),
	}
}

func (m *ConnectionManager) AddClient(c *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[c.ID] = c
	c.Manager = m
}

func (m *ConnectionManager) AssignRoom(c *Client, roomID string) error {
	m.Lock()
	defer m.Unlock()

	if slices.Contains(m.rooms[roomID], c) {
		return fmt.Errorf("Client is already in the room")
	}

	m.rooms[roomID] = append(m.rooms[roomID], c)
	c.RoomID = roomID

	return nil
}

func (m *ConnectionManager) RemoveClient(c *Client) {
	m.Lock()
	defer m.Unlock()

	delete(m.clients, c.ID)
	close(c.egress)

	room, ok := m.rooms[c.RoomID]
	if ok {
		m.rooms[c.RoomID] = slices.DeleteFunc(room, func(current *Client) bool {
			if current == c {
				return true
			}
			return false
		})
		m.onLeave(c)

		// TODO: Clean up empty rooms
	}
}

func (m *ConnectionManager) RegisterEventHandler(event string, handler EventHandler) {
	m.handlers[event] = handler
}

func (m *ConnectionManager) handleMessage(client *Client, message MessageIncoming) {
	handler, ok := m.handlers[message.Type]
	if !ok {
		log.Printf("Unhandled message type %q", message.Type)
		return
	}

	handler(client, message)
}

func (m *ConnectionManager) Broadcast(roomID string, message MessageOutgoing) {
	room, ok := m.rooms[roomID]
	if !ok {
		log.Println("Broadcast to non-existent room")
		return
	}

	for i := range room {
		room[i].Send(message)
	}
}

func (m *ConnectionManager) BroadcastFunc(roomID string, sendFunc func(c *Client)) {
	room, ok := m.rooms[roomID]
	if !ok {
		log.Println("Broadcast to non-existent room")
		return
	}

	for i := range room {
		sendFunc(room[i])
	}
}
