package main

import (
	"encoding/json"

	"github.com/gorilla/websocket"

	t "stmsh/pkg/templates"
	"stmsh/pkg/ws"
)

type JsonSerializer struct{}

func (s *JsonSerializer) Serialize(message ws.MessageOutgoing) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage
	msg, err := json.Marshal(message)
	if err == nil {
		serialized = append(serialized, msg)
	}

	return
}

type HtmxSerializer struct{}

func (s *HtmxSerializer) Serialize(message ws.MessageOutgoing) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage

	switch event := message.(type) {
	case EventRoomInit:
		serialized = append(serialized, t.Render("time", event.Time))
		serialized = append(serialized, t.Render("user", event.User))

		switch event.Stage {
		case StageLobby:
			serialized = append(serialized, t.Render("actions", event.User))
			serialized = append(serialized, t.Render("stage_lobby", event))
		case StageVoting:
			serialized = append(serialized, t.Render("actions", event.User))
			serialized = append(serialized, t.Render("stage_voting", event))
		case StageResults:
			serialized = append(serialized, t.Render("actions_results", event.User))
			serialized = append(serialized, t.Render("stage_results", event))
			serialized = append(serialized, t.Render("results_winners", event.Winners))
			serialized = append(serialized, t.Render("results_others", event.Others))
		}

	case EventPlayersChanged:
		serialized = append(serialized, t.Render("players", event))

	case EventPlayerUpdated:
		serialized = append(serialized, t.Render("user", event))
		serialized = append(serialized, t.Render("actions", event))

	case EventStageVoting:
		serialized = append(serialized, t.Render("stage_voting", event))

	case EventVoteRegistered:
		serialized = append(serialized, t.Render("candidates", event.CandidatesLeft))

	case EventRoomTime:
		serialized = append(serialized, t.Render("time", event.Time))
	case EventTimerSet:
		serialized = append(serialized, t.Render("time", event.Time))

	case EventListChanged:
		serialized = append(serialized, t.Render("list", event.List))

	case EventStageResults:
		serialized = append(serialized, t.Render("actions_results", nil))
		serialized = append(serialized, t.Render("stage_results", event))
		serialized = append(serialized, t.Render("results_winners", event.Winners))
		serialized = append(serialized, t.Render("results_others", event.Others))
	}

	return
}
