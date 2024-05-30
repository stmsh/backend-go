package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Conn       *websocket.Conn
	Egress     chan Event
	RoomID     string
	Serializer Serializer
}

func (c *Client) readMessages() {
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure,
			) {
				log.Printf("error: %v", err)
			}

			HandleLeave(c)
			close(c.Egress)
			break
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.ReportError(err)
			continue
		}

		routeMessage(c, msg)
	}
}

func routeMessage(c *Client, msg Message) {
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
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Egress:
			if !ok {
				log.Println("Closing connection after channel got closed")
				return
			}

			messageType, messages := c.Serializer.Serialize(msg)
			for i := range messages {
				c.Conn.WriteMessage(messageType, messages[i])
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.Conn.WriteMessage(websocket.PingMessage, nil)
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
	c.Egress <- err
}

func (c *Client) Send(msg Event) {
	c.Egress <- msg
}
