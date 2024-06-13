package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	"stmsh/pkg/client"
	t "stmsh/pkg/templates"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 10 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var htmxSerializer = &HtmxSerializer{}
var jsonSerializer = &JsonSerializer{}

func main() {
	roomsRepository := NewInMemoryRoomsRepository()

	r := chi.NewRouter()
	err := godotenv.Load(".env.local", ".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	fs := http.FileServer(http.Dir("./public"))
	r.Handle("/public/*", http.StripPrefix("/public/", fs))

	r.Get("/search", HandleMovieQuery)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("name")
		if err != nil {
			http.Redirect(w, r, "/first-time", http.StatusTemporaryRedirect)
			return
		}

		w.Write(t.Render("index.html", nil))
	})

	r.Group(func(r chi.Router) {
		r.Get("/first-time", func(w http.ResponseWriter, r *http.Request) {
			w.Write(t.Render("first-time.html", nil))
		})

		r.Post("/first-time", func(w http.ResponseWriter, r *http.Request) {
			name := r.FormValue("name")
			w.Header().Add("set-cookie", "name="+name)
			w.Header().Add("hx-redirect", "/")
		})
	})

	r.Post("/create", func(w http.ResponseWriter, r *http.Request) {
		newRoom := NewRoom()
		roomsRepository.Add(newRoom)

		w.Header().Add("HX-Redirect", fmt.Sprintf("/room/%s", newRoom.ID))
	})

	r.Get("/room/{id}", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("id")
		_, err := r.Cookie("clientID")
		if err != nil {
			w.Header().Add("set-cookie", "clientID="+uuid.NewString())
		}

		room := roomsRepository.Find(roomID)
		if room == nil {
			w.Write(t.Render("404.html", nil))
			return
		}

		w.Write(t.Render("room", room))
	})

	handlers := NewHandlers(roomsRepository)

	manager := client.NewConnectionManager(handlers.HandleLeave)
	EnsureRoom := func(handler client.EventHandler) client.EventHandler {
		return func(c *client.Client, m client.MessageIncoming) {
			if c.RoomID == "" {
				c.ReportError(fmt.Errorf("Join room first"))
				return
			}

			handler(c, m)
		}
	}
	manager.RegisterEventHandler(MessageTypeJoin, handlers.HandleJoin)
	manager.RegisterEventHandler(MessageTypeUserToggleReady, EnsureRoom(handlers.HandleToggleReady))
	manager.RegisterEventHandler(MessageTypeNextStage, EnsureRoom(handlers.HandleChangeStage))
	manager.RegisterEventHandler(MessageTypeSetTimer, EnsureRoom(handlers.HandleSetTimer))
	manager.RegisterEventHandler(MessageTypeListAdd, EnsureRoom(handlers.HandleListAdd))
	manager.RegisterEventHandler(MessageTypeListRemove, EnsureRoom(handlers.HandleListRemove))
	manager.RegisterEventHandler(MessageTypeVote, EnsureRoom(handlers.HandleVote))

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade failed: ", err)
			return
		}

		isHtmx := r.URL.Query().Get("htmx") == "true"

		var serializer client.Serializer
		if isHtmx {
			serializer = htmxSerializer
		} else {
			serializer = jsonSerializer
		}
		client := client.NewClient(conn, manager, serializer)
		clientID := r.URL.Query().Get("clientID")
		if clientID != "" {
			client.ID = clientID
		}

		manager.AddClient(client)

		go client.WriteMessages()
		go client.ReadMessages()
	})

	go roomsRepository.RunRoomTimer(manager)
	go roomsRepository.RunRoomCleanup(manager)

	http.ListenAndServe(":8080", r)
}
