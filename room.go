package main

import (
	"log"
	"stmsh/client"
	"time"

	"github.com/google/uuid"
)

type Player struct {
	ID    string
	Name  string
	Ready bool
}

func NewPlayer(name string) *Player {
	return &Player{
		ID:   uuid.NewString(),
		Name: name,
	}
}

type RoomStage string

const (
	StageLobby   = "lobby"
	StageVoting  = "voting"
	StageResults = "results"
)

type ListItem struct {
	ID          string
	Title       string
	Overview    string
	Rating      float32
	ReleaseDate time.Time
	PosterPath  string
}

type Candidate struct {
	ListItem

	SuggestedBy string
	Score       int
	Votes       []string
}

type Room struct {
	ID                   string
	Host                 *Player
	Stage                RoomStage
	Time                 time.Duration
	ScheduledForDeletion bool
	Players              map[string]*Player
	Lists                map[string][]ListItem
	Candidates           []Candidate
}

var rooms = make(map[string]*Room, 100)

func NewRoom() *Room {
	return &Room{
		ID:    uuid.NewString(),
		Time:  0,
		Stage: StageLobby,
		Host:  nil,

		Players:    make(map[string]*Player),
		Lists:      make(map[string][]ListItem),
		Candidates: nil,
	}
}

func RunRoomCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	deleteCount := 0

	for {
		<-ticker.C
		log.Println("Running cleanup")

		for id, room := range rooms {
			if room.ScheduledForDeletion {
				deleteCount++
				delete(rooms, id)
			}
		}

		log.Printf("Rooms deleted: %d", deleteCount)
		deleteCount = 0
	}
}

func RunRoomTimer(manager *client.ConnectionManager) {
	ticker := time.NewTicker(1 * time.Second)

	for {
		<-ticker.C
		for _, r := range rooms {
			if r.Time <= 0 {
				continue
			}

			r.Time = r.Time - 1*time.Second
			manager.Broadcast(r.ID, NewEventRoomTime(r))
		}
	}
}
