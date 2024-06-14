package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"stmsh/pkg/ws"
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

type Handlers struct {
	rooms RoomsRepository
}

func NewHandlers(repo RoomsRepository) *Handlers {
	return &Handlers{
		rooms: repo,
	}
}

func (h *Handlers) HandleJoin(sender *ws.Client, msg ws.MessageIncoming) {
	var payload MessageJoin
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	err := h.rooms.Update(payload.RoomID, func(r *Room) error {
		sender.Manager.AssignRoom(sender, payload.RoomID)

		newPlayer := Player{
			ID:   sender.ID,
			Name: payload.Name,
		}

		if len(r.Players) == 0 {
			r.HostID = newPlayer.ID
		}
		r.Players[sender.ID] = newPlayer

		sender.Send(NewEventRoomInit(newPlayer, *r))
		playerJoined := NewEventPlayerJoined(newPlayer)
		playersChanged := NewEventPlayersChanged(*r)
		sender.Manager.BroadcastFunc(r.ID, func(c *ws.Client) {
			if c != sender {
				c.Send(playerJoined)
			}
			c.Send(playersChanged)
		})

		if r.ScheduledForDeletion {
			r.ScheduledForDeletion = false
		}

		return nil
	})

	if err != nil {
		sender.ReportError(err)
		return
	}
}

func (h *Handlers) HandleToggleReady(sender *ws.Client, msg ws.MessageIncoming) {
	var payload MessageUserToggleReady
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	h.rooms.Update(sender.RoomID, func(r *Room) error {
		p := r.Players[sender.ID]
		p.Ready = payload.Ready
		r.Players[sender.ID] = p

		sender.Send(NewPlayerUpdatedEvent(p, *r))
		sender.Manager.Broadcast(r.ID, NewEventPlayersChanged(*r))

		return nil
	})
}

func (h *Handlers) HandleLeave(sender *ws.Client) {
	h.rooms.Update(sender.RoomID, func(room *Room) error {
		delete(room.Players, sender.ID)

		if room.HostID == sender.ID {
			var nextHostID string
			for _, p := range room.Players {
				nextHostID = p.ID
				break
			}

			room.HostID = nextHostID

			if room.HostID != "" {
				hostChanged := NewHostChangedEvent(room.Players[room.HostID])
				sender.Manager.BroadcastFunc(room.ID, func(c *ws.Client) {
					c.Send(hostChanged)
					// notify new host that its data changed
					if c.ID == room.HostID {
						c.Send(NewPlayerUpdatedEvent(room.Players[c.ID], *room))
					}
				})
			}
		}

		sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(*room))

		if len(room.Players) == 0 {
			room.ScheduledForDeletion = true
		}

		return nil
	})
}

func (h *Handlers) HandleChangeStage(sender *ws.Client, _ ws.MessageIncoming) {
	err := h.rooms.Update(sender.RoomID, func(room *Room) error {
		user := room.Players[sender.ID]
		if user.ID != room.HostID {
			return fmt.Errorf("Only host can change stage")
		}

		if room.Stage == StageResults {
			return fmt.Errorf("Can't change stage. Final stage reached")
		}

		room.Stage = RoomStage(nextStageMap[string(room.Stage)])

		switch room.Stage {
		case StageVoting:
			// Currently need to emit player updated event to update actions
			// Think of different strategy for updating actions
			for _, p := range room.Players {
				p.Ready = false
				room.Players[p.ID] = p
			}
			sender.Manager.BroadcastFunc(room.ID, func(c *ws.Client) {
				c.Send(NewPlayerUpdatedEvent(room.Players[c.ID], *room))
			})
			sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(*room))

			room.Candidates = collectCandidates(*room)
			sender.Manager.Broadcast(room.ID, NewEventStageVoting(*room))

		case StageResults:
			sender.Manager.Broadcast(room.ID, NewEventStageResults(*room))
		}

		return nil
	})

	if err != nil {
		sender.ReportError(err)
	}
}

func (h *Handlers) HandleSetTimer(sender *ws.Client, msg ws.MessageIncoming) {
	err := h.rooms.Update(sender.RoomID, func(room *Room) error {
		user := room.Players[sender.ID]
		if user.ID != room.HostID {
			return fmt.Errorf("Only host can change stage")
		}

		var payload MessageSetTimer
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}

		room.Time = time.Duration(payload.TimeInSeconds) * time.Second
		sender.Manager.Broadcast(room.ID, NewTimerSetEvent(*room))

		return nil
	})

	if err != nil {
		sender.ReportError(err)
	}
}

func (h *Handlers) HandleListAdd(sender *ws.Client, msg ws.MessageIncoming) {
	var payload MessageListAdd
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	err := h.rooms.Update(sender.RoomID, func(room *Room) error {
		user := room.Players[sender.ID]
		newItemID := strconv.Itoa(payload.ID)

		if slices.ContainsFunc(room.Lists[user.ID], func(item ListItem) bool {
			return item.ID == newItemID
		}) {
			return fmt.Errorf("Item already in the list")
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

		return nil
	})

	if err != nil {
		sender.ReportError(err)
	}
}

func (h *Handlers) HandleListRemove(sender *ws.Client, msg ws.MessageIncoming) {
	var payload MessageListRemove
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	err := h.rooms.Update(sender.RoomID, func(room *Room) error {
		user := room.Players[sender.ID]

		updatedList := slices.DeleteFunc(room.Lists[user.ID], func(v ListItem) bool {
			return v.ID == payload.ID
		})

		room.Lists[user.ID] = updatedList
		sender.Send(NewEventListChanged(updatedList))

		return nil
	})

	if err != nil {
		sender.ReportError(err)
	}
}

func (h *Handlers) HandleVote(sender *ws.Client, msg ws.MessageIncoming) {
	var payload MessageVote
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sender.ReportError(err)
		return
	}

	err := h.rooms.Update(sender.RoomID, func(room *Room) error {
		if room.Stage != StageVoting {
			return fmt.Errorf("Not in voting stage")
		}

		user := room.Players[sender.ID]
		for i, candidate := range room.Candidates {
			if candidate.ID == payload.ID {
				if slices.Contains(candidate.Votes, user.ID) {
					return fmt.Errorf("Already voted for %s", candidate.ID)
				}

				room.Candidates[i].Votes = append(room.Candidates[i].Votes, user.ID)
				if payload.Vote {
					room.Candidates[i].Score++
				} else {
					room.Candidates[i].Score--
				}
			}
		}

		event := NewEventVoteRegistered(user, *room)
		if len(event.CandidatesLeft) == 0 {
			user.Ready = true
			room.Players[user.ID] = user
			sender.Send(NewPlayerUpdatedEvent(user, *room))
			sender.Manager.Broadcast(room.ID, NewEventPlayersChanged(*room))
		}
		sender.Send(event)

		return nil
	})

	if err != nil {
		sender.ReportError(err)
	}
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
		Total      int            `json:"total"`
		Candidates []candidate    `json:"candidates"`
		Winners    []resultsEntry `json:"winners"`
		Others     []resultsEntry `json:"others"`
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
		Total      int         `json:"total"`
		Candidates []candidate `json:"candidates"`
	}

	EventVoteRegistered struct {
		Type           string      `json:"type"`
		Total          int         `json:"total"`
		CandidatesLeft []candidate `json:"candidates"`
	}

	resultsEntry struct {
		listItem
		Score int `json:"score"`
	}

	EventStageResults struct {
		Type    string         `json:"type"`
		Winners []resultsEntry `json:"winners"`
		Others  []resultsEntry `json:"others"`
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

const (
	LimitCandidates = 5
)

func NewEventRoomInit(user Player, room Room) EventRoomInit {
	players := make([]player, 0, len(room.Players))

	for _, v := range room.Players {
		players = append(players, player{
			ID:     v.ID,
			Name:   v.Name,
			Ready:  v.Ready,
			IsHost: v.ID == room.HostID,
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

	winners, others := collectResults(room)
	remaining := collectRemainingCandidates(user, room)
	candidates := transformCandidates(tail(remaining, LimitCandidates))

	return EventRoomInit{
		Type: EventTypeRoomInit,
		ID:   room.ID,
		User: player{
			ID:     user.ID,
			Name:   user.Name,
			Ready:  user.Ready,
			IsHost: room.HostID == user.ID,
		},
		Time:       room.Time,
		List:       list,
		Stage:      room.Stage,
		Players:    players,
		Total:      len(remaining),
		Candidates: candidates,
		Winners:    winners,
		Others:     others,
	}
}

func NewEventPlayerJoined(p Player) EventPlayerJoined {
	return EventPlayerJoined{
		Type: EventTypePlayerJoined,
		ID:   p.ID,
		Name: p.Name,
	}
}

func NewEventPlayersChanged(room Room) EventPlayersChanged {
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
			IsHost: v.ID == room.HostID,
		})
	}

	return EventPlayersChanged{
		Type:    EventTypePlayersChanged,
		Ready:   ready,
		Total:   total,
		Players: players,
	}
}

func NewPlayerUpdatedEvent(p Player, room Room) EventPlayerUpdated {
	return EventPlayerUpdated{
		Type:   EventTypePlayerUpdated,
		ID:     p.ID,
		Name:   p.Name,
		Ready:  p.Ready,
		IsHost: p.ID == room.HostID,
	}
}

func NewHostChangedEvent(newHost Player) EventHostChanged {
	return EventHostChanged{
		ID:   newHost.ID,
		Name: newHost.Name,
	}
}

var nextStageMap = map[string]string{
	StageLobby:   StageVoting,
	StageVoting:  StageResults,
	StageResults: "",
}

func NewTimerSetEvent(room Room) EventTimerSet {
	return EventTimerSet{
		Type: EventTypeTimerSet,
		Time: room.Time,
	}
}

func NewEventRoomTime(room Room) EventRoomTime {
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

func collectCandidates(room Room) []Candidate {
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

func NewEventStageVoting(room Room) EventStageVoting {
	return EventStageVoting{
		Type:       EventTypeStageVoting,
		Total:      len(room.Candidates),
		Candidates: transformCandidates(tail(room.Candidates, LimitCandidates)),
	}
}

func collectRemainingCandidates(player Player, room Room) []Candidate {
	remaining := make([]Candidate, 0, len(room.Candidates))
	for _, c := range room.Candidates {
		if slices.Contains(c.Votes, player.ID) {
			continue
		}
		remaining = append(remaining, c)
	}

	return remaining
}

func NewEventVoteRegistered(voter Player, room Room) EventVoteRegistered {
	remaining := collectRemainingCandidates(voter, room)
	candidates := transformCandidates(
		tail(remaining, LimitCandidates))

	return EventVoteRegistered{
		Type:           EventTypeVoteRegistered,
		Total:          len(remaining),
		CandidatesLeft: candidates,
	}
}

func collectResults(room Room) ([]resultsEntry, []resultsEntry) {
	results := make([]resultsEntry, len(room.Candidates))
	for i, candidate := range room.Candidates {
		results[i] = resultsEntry{
			listItem: listItem(candidate.ListItem),
			Score:    candidate.Score,
		}
	}

	slices.SortFunc(results, func(a, b resultsEntry) int {
		return b.Score - a.Score
	})

	var maxScore int
	if len(results) > 0 {
		maxScore = results[0].Score
	}

	winners := make([]resultsEntry, 0, len(results))
	others := make([]resultsEntry, 0, len(results))
	for _, item := range results {
		if item.Score == maxScore {
			winners = append(winners, item)
		} else {
			others = append(others, item)
		}
	}

	return winners, others
}

func NewEventStageResults(room Room) EventStageResults {
	winners, others := collectResults(room)

	return EventStageResults{
		Type:    EventTypeStageResults,
		Winners: winners,
		Others:  others,
	}
}

func tail[S ~[]E, E any](s S, n int) S {
	start := max(len(s)-n-1, 0)
	return s[start:]
}
