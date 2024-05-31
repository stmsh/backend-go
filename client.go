package main

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	BaseClientsCount = 100
	BaseRoomsCount   = 10
)

type Client struct {
	ID     string
	RoomID string

	conn   *websocket.Conn
	egress chan Event

	Serializer Serializer
	Manager    *ConnectionManager
}

type ConnectionManager struct {
	sync.RWMutex
	clients map[string]*Client
	rooms   map[string][]*Client

	// Client will be removed from room before onLeave call.
	// No messages will be delivered to disconnected client.
	onLeave func(c *Client)
}

func NewConnectionManager(onLeave func(c *Client)) *ConnectionManager {
	return &ConnectionManager{
		clients: make(map[string]*Client, BaseClientsCount),
		rooms:   make(map[string][]*Client, BaseRoomsCount),
		onLeave: onLeave,
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

func (c *Client) readMessages() {
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

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.ReportError(err)
			continue
		}

		c.Manager.routeMessage(c, msg)
	}
}

func (m *ConnectionManager) routeMessage(c *Client, msg Message) {
	if c.RoomID == "" {
		if msg.Type == MessageTypeJoin {
			HandleJoin(c, msg)
		}
		return
	}

	switch msg.Type {
	case MessageTypeUserToggleReady:
		HandleToggleReady(c, msg)
	case MessageTypeNextStage:
		HandleChangeStage(c)
	case MessageTypeSetTimer:
		HandleSetTimer(c, msg)
	case MessageTypeListAdd:
		HandleListAdd(c, msg)
	case MessageTypeListRemove:
		HandleListRemove(c, msg)
	case MessageTypeVote:
		HandleVote(c, msg)
	default:
		log.Printf("Unrecognized message type: %s", msg.Type)
	}
}

type Serializer interface {
	Serialize(Event) (int, [][]byte)
}

func (c *Client) writeMessages() {
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

type JsonSerializer struct{}

func (s *JsonSerializer) Serialize(event Event) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage
	msg, err := json.Marshal(event)
	if err == nil {
		serialized = append(serialized, msg)
	}

	return
}

type HtmxSerializer struct{}

func (s *HtmxSerializer) Serialize(event Event) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage

	switch event := event.(type) {
	case EventRoomInit:
		serialized = append(serialized, Render("time", event.Time))
		serialized = append(serialized, Render("user", event.User))
		serialized = append(serialized, Render("actions", event.User))

		switch event.Stage {
		case StageLobby:
			serialized = append(serialized, Render("stage_lobby", event))
		case StageVoting:
			serialized = append(serialized, Render("stage_voting", event))
		case StageResults:
			serialized = append(serialized, Render("stage_results", event))
		}

	case EventPlayersChanged:
		serialized = append(serialized, Render("players", event))

	case EventPlayerUpdated:
		serialized = append(serialized, Render("user", event))
		serialized = append(serialized, Render("actions", event))

	case EventStageVoting:
		serialized = append(serialized, Render("stage_voting", event))

	case EventVoteRegistered:
		serialized = append(serialized, Render("candidates", event.CandidatesLeft))

	case EventRoomTime:
		serialized = append(serialized, Render("time", event.Time))
	case EventTimerSet:
		serialized = append(serialized, Render("time", event.Time))

	case EventListChanged:
		serialized = append(serialized, Render("list", event.List))

	case EventStageResults:
		serialized = append(serialized, Render("stage_results", event))

	default:
		log.Printf("In HtmxSerializer.Serialize. Unrecognized event: %v", event)
	}

	return
}

func (c *Client) ReportError(err error) {
	c.egress <- err
}

func (c *Client) Send(msg Event) {
	c.egress <- msg
}
