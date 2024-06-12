package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"stmsh/pkg/client"
)

type Player struct {
	ID    string
	Name  string
	Ready bool
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
	HostID               string
	Stage                RoomStage
	Time                 time.Duration
	ScheduledForDeletion bool
	Players              map[string]Player
	Lists                map[string][]ListItem
	Candidates           []Candidate
}

func NewRoom() Room {
	return Room{
		ID:     uuid.NewString(),
		Time:   0,
		Stage:  StageLobby,
		HostID: "",

		Players:    make(map[string]Player),
		Lists:      make(map[string][]ListItem),
		Candidates: nil,
	}
}

type RoomsRepository interface {
	Add(room Room)
	Find(id string) *Room
	Update(id string, updateFn func(*Room) (*Room, error)) error
	Delete(id string)
}

type InMemoryRoomsRepository struct {
	lock  *sync.RWMutex
	rooms map[string]Room
}

func NewInMemoryRoomsRepository() *InMemoryRoomsRepository {
	return &InMemoryRoomsRepository{
		lock:  &sync.RWMutex{},
		rooms: make(map[string]Room),
	}
}

func (r *InMemoryRoomsRepository) Add(room Room) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.rooms[room.ID] = room
}

func (r *InMemoryRoomsRepository) Find(id string) *Room {
	r.lock.RLock()
	defer r.lock.RUnlock()

	room, ok := r.rooms[id]
	if !ok {
		return nil
	}

	return &room
}

func (r *InMemoryRoomsRepository) Update(id string, updateFn func(room *Room) (*Room, error)) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	room, ok := r.rooms[id]
	if !ok {
		return fmt.Errorf("Room doesn't exist")
	}

	updatedRoom, err := updateFn(&room)
	if err != nil {
		return err
	}

	r.rooms[room.ID] = *updatedRoom

	return nil
}

func (r *InMemoryRoomsRepository) Delete(id string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	delete(r.rooms, id)
}

func (r *InMemoryRoomsRepository) RunRoomCleanup(manager *client.ConnectionManager) {
	ticker := time.NewTicker(5 * time.Minute)
	deleteCount := 0

	for {
		<-ticker.C
		log.Println("Running cleanup")

		r.lock.Lock()
		for id, room := range r.rooms {
			if room.ScheduledForDeletion {
				deleteCount++
				manager.DeleteRoom(id)
				delete(r.rooms, id)
			}
		}
		r.lock.Unlock()

		log.Printf("Rooms deleted: %d", deleteCount)
		deleteCount = 0
	}
}

func (r *InMemoryRoomsRepository) RunRoomTimer(manager *client.ConnectionManager) {
	ticker := time.NewTicker(1 * time.Second)

	for {
		<-ticker.C
		r.lock.Lock()
		for _, room := range r.rooms {
			if room.Time <= 0 {
				continue
			}

			room.Time = room.Time - 1*time.Second
			manager.Broadcast(room.ID, NewEventRoomTime(room))
		}
		r.lock.Unlock()
	}
}
