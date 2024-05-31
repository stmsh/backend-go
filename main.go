package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
			if name == "" {
				fmt.Fprint(w, "<p>Please enter name</p>")
				return
			}

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

		if roomID == "" {
			fmt.Fprint(w, "<p>Room id is required</p>")
			return
		}

		room, ok := rooms[roomID]
		if !ok {
			fmt.Fprintf(w, "<p>Room does not exist</p>")
			return
		}

		w.Write(Render("room.html", room))
	})

	connectionManager := NewConnectionManager(HandleLeave)

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade failed: ", err)
			return
		}

		client := &Client{
			ID:     uuid.NewString(),
			conn:   conn,
			egress: make(chan Event),
		}

		isHtmx := r.URL.Query().Get("htmx") == "true"
		if isHtmx {
			client.Serializer = htmxSerializer
		} else {
			client.Serializer = jsonSerializer
		}

		connectionManager.AddClient(client)

		go client.writeMessages()
		go client.readMessages()
	})

	go RunRoomTimer()
	go RunRoomCleanup()

	http.ListenAndServe(":8080", r)
}
