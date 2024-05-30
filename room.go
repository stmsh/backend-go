package main

import (
	"log"
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
	Name string
}

type Candidate struct {
	Name        string
	SuggestedBy string
	Score       int
	Votes       []string
}

// Room is directly tied to client implementation
type Room struct {
	ID                   string
	Host                 *Client
	Stage                RoomStage
	Time                 time.Duration
	ScheduledForDeletion bool
	Players              map[*Client]*Player
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

		Players:    make(map[*Client]*Player),
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

func RunRoomTimer() {
	// TODO: Fix data race
	// ticker := time.NewTicker(1 * time.Second)
	//
	// for {
	// 	<-ticker.C
	// 	for _, r := range rooms {
	// 		if r.Time <= 0 {
	// 			continue
	// 		}
	//
	// 		r.Time = r.Time - 1*time.Second
	// 		broad := NewEventRoomTime(r)
	// 		for c := range r.Players {
	// 			c.Send(broad)
	// 		}
	// 	}
	// }
}
