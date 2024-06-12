package main

import (
	"fmt"
	"log"
	"net/http"
	"stmsh/client"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
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

		w.Write(Render("index.html", nil))
	})

	r.Group(func(r chi.Router) {
		r.Get("/first-time", func(w http.ResponseWriter, r *http.Request) {
			w.Write(Render("first-time.html", nil))
		})

		r.Post("/first-time", func(w http.ResponseWriter, r *http.Request) {
			name := r.FormValue("name")
			w.Header().Add("set-cookie", "name="+name)
			w.Header().Add("hx-redirect", "/")
		})
	})

	r.Post("/create", func(w http.ResponseWriter, r *http.Request) {
		newRoom := NewRoom()
		rooms[newRoom.ID] = newRoom

		w.Header().Add("HX-Redirect", fmt.Sprintf("/room/%s", newRoom.ID))
	})

	r.Get("/room/{id}", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("id")

		room, ok := rooms[roomID]
		if !ok {
			w.Write(Render("404.html", nil))
			return
		}

		w.Write(Render("room", room))
	})

	manager := client.NewConnectionManager(HandleLeave)
	EnsureRoom := func(handler client.EventHandler) client.EventHandler {
		return func(c *client.Client, m client.MessageIncoming) {
			if c.RoomID == "" {
				c.ReportError(fmt.Errorf("Join room first"))
				return
			}

			handler(c, m)
		}
	}
	manager.RegisterEventHandler(MessageTypeJoin, HandleJoin)
	manager.RegisterEventHandler(MessageTypeUserToggleReady, EnsureRoom(HandleToggleReady))
	manager.RegisterEventHandler(MessageTypeNextStage, EnsureRoom(HandleChangeStage))
	manager.RegisterEventHandler(MessageTypeSetTimer, EnsureRoom(HandleSetTimer))
	manager.RegisterEventHandler(MessageTypeListAdd, EnsureRoom(HandleListAdd))
	manager.RegisterEventHandler(MessageTypeListRemove, EnsureRoom(HandleListRemove))
	manager.RegisterEventHandler(MessageTypeVote, EnsureRoom(HandleVote))

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
		manager.AddClient(client)

		go client.WriteMessages()
		go client.ReadMessages()
	})

	go RunRoomTimer(manager)
	go RunRoomCleanup()

	http.ListenAndServe(":8080", r)
}
