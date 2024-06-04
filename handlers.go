package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"stmsh/client"
	"strconv"
	"time"
)

type (
	MessageJoin struct {
		Name   string `json:"name"`
		RoomID string `json:"roomid"`
	}

	MessageUserToggleReady struct {
		Ready bool `json:"ready"`
	}

	MessageChangeStage struct {
		Stage string `json:"stage"`
	}

	MessageSetTimer struct {
		TimeInSeconds int `json:"time_in_seconds"`
	}

	MessageListAdd struct {
		TMDBMovie
	}

	MessageListRemove struct {
		ID string `json:"id"`
	}

	MessageVote struct {
		ID   string `json:"id"`
		Vote bool   `json:"vote"`
	}
)

const (
	MessageTypeJoin            = "join"
	MessageTypeUserToggleReady = "ready"
	MessageTypeNextStage       = "next_stage"
	MessageTypeSetTimer        = "set_timer"
	MessageTypeListAdd         = "list_add"
	MessageTypeListRemove      = "list_remove"
	MessageTypeVote            = "vote"
)

func HandleJoin(sender *client.Client, msg client.MessageIncoming) {
	var payload MessageJoin
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room, ok := rooms[payload.RoomID]
	if !ok {
		sender.ReportError(fmt.Errorf("Room doesn't exist"))
		return
	}
	sender.Manager.AssignRoom(sender, payload.RoomID)

	newPlayer := &Player{
		ID:   sender.ID,
		Name: payload.Name,
	}

	if len(room.Players) == 0 {
		room.Host = newPlayer
	}
	room.Players[sender.ID] = newPlayer

	sender.Send(NewEventRoomInit(newPlayer, room))

	playerJoined := NewEventPlayerJoined(newPlayer)
	playersChanged := NewEventPlayersChanged(room)
	sender.Manager.BroadcastFunc(room.ID, func(c *client.Client) {
		if c != sender {
			c.Send(playerJoined)
		}
		c.Send(playersChanged)
	})
}

func HandleToggleReady(sender *client.Client, msg client.MessageIncoming) {
	var payload MessageUserToggleReady
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room := rooms[sender.RoomID]
	p := room.Players[sender.ID]
	p.Ready = payload.Ready

	sender.Send(NewPlayerUpdatedEvent(p, room))
	sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(room))
}

func HandleLeave(sender *client.Client) {
	room, ok := rooms[sender.RoomID]
	if !ok {
		return
	}
	deletedUser := room.Players[sender.ID]
	delete(room.Players, sender.ID)

	if room.Host == deletedUser {
		var nextHost *Player
		for _, p := range room.Players {
			nextHost = p
			break
		}

		room.Host = nextHost

		if room.Host != nil {
			hostChanged := NewHostChangedEvent(room)
			sender.Manager.BroadcastFunc(room.ID, func(c *client.Client) {
				c.Send(hostChanged)
				// notify new host that its data changed
				if c.ID == room.Host.ID {
					c.Send(NewPlayerUpdatedEvent(room.Players[c.ID], room))
				}
			})
		}
	}

	sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(room))

	if len(room.Players) == 0 {
		room.ScheduledForDeletion = true
	}
}

func HandleChangeStage(sender *client.Client, _ client.MessageIncoming) {
	room := rooms[sender.RoomID]
	user := room.Players[sender.ID]
	if user != room.Host {
		sender.ReportError(fmt.Errorf("Only host can change stage"))
		return
	}

	if room.Stage == StageResults {
		sender.ReportError(
			fmt.Errorf("Can't change stage. Final stage reached"))
		return
	}

	room.Stage = RoomStage(nextStageMap[string(room.Stage)])

	// Currently need to emit player updated event to update actions
	// Think of different strategy for updating actions
	for _, p := range room.Players {
		p.Ready = false
		sender.Send(NewPlayerUpdatedEvent(user, room))
	}
	sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(room))

	switch room.Stage {
	case StageVoting:
		room.Candidates = collectCandidates(room)
		sender.Manager.Broadcast(room.ID, NewEventStageVoting(room))
	case StageResults:
		sender.Manager.Broadcast(room.ID, NewEventStageResults(room))
	}
}

func HandleSetTimer(sender *client.Client, msg client.MessageIncoming) {
	room := rooms[sender.RoomID]
	user := room.Players[sender.ID]
	if user != room.Host {
		sender.ReportError(fmt.Errorf("Only host can change stage"))
		return
	}

	var payload MessageSetTimer
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room.Time = time.Duration(payload.TimeInSeconds) * time.Second
	sender.Manager.Broadcast(room.ID, NewTimerSetEvent(room))
}

func HandleListAdd(sender *client.Client, msg client.MessageIncoming) {
	var payload MessageListAdd
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room := rooms[sender.RoomID]
	user := room.Players[sender.ID]

	newItemID := strconv.Itoa(payload.ID)

	if slices.ContainsFunc(room.Lists[user.ID], func(item ListItem) bool {
		return item.ID == newItemID
	}) {
		sender.ReportError(fmt.Errorf("Item already in the list"))
		return
	}

	listItem := ListItem{
		ID:         newItemID,
		Title:      payload.Title,
		Overview:   payload.Overview,
		Rating:     payload.Rating,
		PosterPath: payload.PosterPath,
	}
	listItem.ReleaseDate, _ = time.Parse("2006-01-02", payload.ReleaseDate)

	room.Lists[user.ID] = append(room.Lists[user.ID], listItem)

	listChanged := NewEventListChanged(room.Lists[user.ID])
	sender.Send(listChanged)
}

func HandleListRemove(sender *client.Client, msg client.MessageIncoming) {
	var payload MessageListRemove
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room := rooms[sender.RoomID]
	user := room.Players[sender.ID]

	updatedList := slices.DeleteFunc(room.Lists[user.ID], func(v ListItem) bool {
		return v.ID == payload.ID
	})

	room.Lists[user.ID] = updatedList
	sender.Send(NewEventListChanged(updatedList))
}

func HandleVote(sender *client.Client, msg client.MessageIncoming) {
	var payload MessageVote
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	room := rooms[sender.RoomID]
	if room.Stage != StageVoting {
		sender.ReportError(fmt.Errorf("Not in voting stage"))
		return
	}

	user := room.Players[sender.ID]
	for i, candidate := range room.Candidates {
		if candidate.ID == payload.ID {
			if slices.Contains(candidate.Votes, user.ID) {
				sender.ReportError(
					fmt.Errorf("Already voted for %s", candidate.ID))
				return
			}

			room.Candidates[i].Votes = append(room.Candidates[i].Votes, user.ID)
			if payload.Vote {
				room.Candidates[i].Score++
			} else {
				room.Candidates[i].Score--
			}
		}
	}

	event := NewEventVoteRegistered(user, room)
	if len(event.CandidatesLeft) == 0 {
		user.Ready = true
		sender.Send(NewPlayerUpdatedEvent(user, room))
		sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(room))
	}
	sender.Send(NewEventVoteRegistered(user, room))
}

type (
	player struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Ready  bool   `json:"ready"`
		IsHost bool   `json:"isHost"`
	}

	listItem struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		Overview    string    `json:"overview"`
		Rating      float32   `json:"rating"`
		ReleaseDate time.Time `json:"release_date"`
		PosterPath  string    `json:"poster_path"`
	}

	candidate struct {
		listItem
		SuggestedBy string `json:"suggestedBy"`
	}
)

type (
	EventRoomInit struct {
		Type       string         `json:"type"`
		ID         string         `json:"id"`
		User       player         `json:"user"`
		Stage      RoomStage      `json:"stage"`
		Time       time.Duration  `json:"time"`
		List       []listItem     `json:"list"`
		Players    []player       `json:"players"`
		Candidates []candidate    `json:"candidates"`
		Results    []resultsEntry `json:"results"`
	}

	EventPlayerJoined struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	EventPlayersChanged struct {
		Type    string   `json:"type"`
		Ready   int      `json:"ready"`
		Total   int      `json:"total"`
		Players []player `json:"players"`
	}

	EventPlayerUpdated struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Name   string `json:"name"`
		Ready  bool   `json:"ready"`
		IsHost bool   `json:"isHost"`
	}

	EventHostChanged struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	EventTimerSet struct {
		Type string        `json:"type"`
		Time time.Duration `json:"time"`
	}

	EventRoomTime struct {
		Type string        `json:"type"`
		Time time.Duration `json:"time"`
	}

	EventListChanged struct {
		Type string     `json:"type"`
		List []listItem `json:"list"`
	}

	EventStageVoting struct {
		Type       string      `json:"type"`
		Candidates []candidate `json:"candidates"`
	}

	EventVoteRegistered struct {
		Type           string      `json:"type"`
		CandidatesLeft []candidate `json:"candidates"`
	}

	resultsEntry struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}

	EventStageResults struct {
		Type    string         `json:"type"`
		Results []resultsEntry `json:"results"`
	}
)

const (
	EventTypeRoomInit       = "room:init"
	EventTypePlayerJoined   = "room:player_joined"
	EventTypePlayersChanged = "room:players_changed"
	EventTypeHostChanged    = "room:host_changed"
	EventTypeTimerSet       = "room:timer_set"
	EventTypeRoomTime       = "room:time"

	EventTypeStageVoting    = "room:stage_voting"
	EventTypeVoteRegistered = "room:vote_registered"
	EventTypeStageResults   = "room:stage_results"

	EventTypePlayerUpdated = "player:update"
	EventTypeListChanged   = "player:list_changed"
)

func NewEventRoomInit(user *Player, room *Room) EventRoomInit {
	players := make([]player, 0, len(room.Players))

	for _, v := range room.Players {
		players = append(players, player{
			ID:     v.ID,
			Name:   v.Name,
			Ready:  v.Ready,
			IsHost: v == room.Host,
		})
	}

	list := make([]listItem, len(room.Lists[user.ID]))

	for i, v := range room.Lists[user.ID] {
		list[i] = listItem{
			ID:          v.ID,
			Title:       v.Title,
			Overview:    v.Overview,
			Rating:      v.Rating,
			ReleaseDate: v.ReleaseDate,
			PosterPath:  v.PosterPath,
		}
	}

	return EventRoomInit{
		Type: EventTypeRoomInit,
		ID:   room.ID,
		User: player{
			ID:     user.ID,
			Name:   user.Name,
			Ready:  user.Ready,
			IsHost: room.Host == user,
		},
		Time:       room.Time,
		List:       list,
		Stage:      room.Stage,
		Players:    players,
		Candidates: transformCandidates(room.Candidates),
		Results:    collectResults(room),
	}
}

func NewEventPlayerJoined(p *Player) EventPlayerJoined {
	return EventPlayerJoined{
		Type: EventTypePlayerJoined,
		ID:   p.ID,
		Name: p.Name,
	}
}

func NewEventPlayersChanged(room *Room) EventPlayersChanged {
	ready := 0
	total := len(room.Players)
	players := make([]player, 0, total)

	for _, v := range room.Players {
		if v.Ready {
			ready++
		}

		players = append(players, player{
			ID:     v.ID,
			Name:   v.Name,
			Ready:  v.Ready,
			IsHost: v == room.Host,
		})
	}

	return EventPlayersChanged{
		Type:    EventTypePlayersChanged,
		Ready:   ready,
		Total:   total,
		Players: players,
	}
}

func NewPlayerUpdatedEvent(p *Player, room *Room) EventPlayerUpdated {
	return EventPlayerUpdated{
		Type:   EventTypePlayerUpdated,
		ID:     p.ID,
		Name:   p.Name,
		Ready:  p.Ready,
		IsHost: room.Host == p,
	}
}

func NewHostChangedEvent(room *Room) EventHostChanged {

	return EventHostChanged{
		ID:   room.Host.ID,
		Name: room.Host.Name,
	}
}

var nextStageMap = map[string]string{
	StageLobby:   StageVoting,
	StageVoting:  StageResults,
	StageResults: "",
}

func NewTimerSetEvent(room *Room) EventTimerSet {
	return EventTimerSet{
		Type: EventTypeTimerSet,
		Time: room.Time,
	}
}

func NewEventRoomTime(room *Room) EventRoomTime {
	return EventRoomTime{
		Type: EventTypeRoomTime,
		Time: room.Time,
	}
}

func NewEventListChanged(list []ListItem) EventListChanged {
	eventList := make([]listItem, len(list))
	for i, v := range list {
		eventList[i] = listItem(v)
	}

	return EventListChanged{
		Type: EventTypeListChanged,
		List: eventList,
	}
}

func collectCandidates(room *Room) []Candidate {
	c := make([]Candidate, 0, 0)

	for id, list := range room.Lists {
		for _, item := range list {
			if !slices.ContainsFunc(c, func(v Candidate) bool {
				return v.ID == item.ID
			}) {
				c = append(c, Candidate{
					ListItem:    item,
					SuggestedBy: id,
					Score:       0,
					Votes:       []string{},
				})
			}
		}
	}

	return c
}

func transformCandidates(candidates []Candidate) []candidate {
	c := make([]candidate, len(candidates))
	for i, v := range candidates {
		c[i] = candidate{
			listItem:    listItem(v.ListItem),
			SuggestedBy: v.SuggestedBy,
		}
	}

	return c
}

func NewEventStageVoting(room *Room) EventStageVoting {
	return EventStageVoting{
		Type:       EventTypeStageVoting,
		Candidates: transformCandidates(room.Candidates),
	}
}

func NewEventVoteRegistered(voter *Player, room *Room) EventVoteRegistered {
	left := make([]Candidate, 0, len(room.Candidates))
	for _, c := range room.Candidates {
		if slices.Contains(c.Votes, voter.ID) {
			continue
		}
		left = append(left, c)
	}

	return EventVoteRegistered{
		Type:           EventTypeVoteRegistered,
		CandidatesLeft: transformCandidates(left),
	}
}

func collectResults(room *Room) []resultsEntry {
	var results []resultsEntry
	for _, candidate := range room.Candidates {
		results = append(results, resultsEntry{
			Name:  candidate.Title,
			Score: candidate.Score,
		})
	}

	slices.SortFunc(results, func(a, b resultsEntry) int {
		return b.Score - a.Score
	})

	return results
}

func NewEventStageResults(room *Room) EventStageResults {
	return EventStageResults{
		Type:    EventTypeStageResults,
		Results: collectResults(room),
	}
}
