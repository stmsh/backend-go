package main

import (
	"encoding/json"

	"github.com/gorilla/websocket"

	"stmsh/pkg/client"
)

type JsonSerializer struct{}

func (s *JsonSerializer) Serialize(message client.MessageOutgoing) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage
	msg, err := json.Marshal(message)
	if err == nil {
		serialized = append(serialized, msg)
	}

	return
}

type HtmxSerializer struct{}

func (s *HtmxSerializer) Serialize(message client.MessageOutgoing) (messageType int, serialized [][]byte) {
	messageType = websocket.TextMessage

	switch event := message.(type) {
	case EventRoomInit:
		serialized = append(serialized, Render("time", event.Time))
		serialized = append(serialized, Render("user", event.User))

		switch event.Stage {
		case StageLobby:
			serialized = append(serialized, Render("actions", event.User))
			serialized = append(serialized, Render("stage_lobby", event))
		case StageVoting:
			serialized = append(serialized, Render("actions", event.User))
			serialized = append(serialized, Render("stage_voting", event))
		case StageResults:
			serialized = append(serialized, Render("actions_results", event.User))
			serialized = append(serialized, Render("stage_results", event))
			serialized = append(serialized, Render("results_winners", event.Winners))
			serialized = append(serialized, Render("results_others", event.Others))
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
		serialized = append(serialized, Render("actions_results", nil))
		serialized = append(serialized, Render("stage_results", event))
		serialized = append(serialized, Render("results_winners", event.Winners))
		serialized = append(serialized, Render("results_others", event.Others))
	}

	return
}
