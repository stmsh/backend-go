package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	t "stmsh/pkg/templates"
	"stmsh/pkg/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var htmxSerializer = &HtmxSerializer{}
var jsonSerializer = &JsonSerializer{}

func init() {
	err := godotenv.Load(".env.local", ".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func IgnorePaths(
	middleware func(http.Handler) http.Handler,
	skipPrefixes ...string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, skipPrefix := range skipPrefixes {
				if strings.HasPrefix(r.URL.Path, skipPrefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			middleware(next).ServeHTTP(w, r)
		})
	}
}

func main() {
	roomsRepository := NewInMemoryRoomsRepository()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(IgnorePaths(middleware.Logger, "/public"))
	r.Use(middleware.Recoverer)
	if os.Getenv("ENABLE_PROFILING") == "true" {
		r.Mount("/debug", middleware.Profiler())
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
		w.Write([]byte(newRoom.ID))
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

	manager := ws.NewConnectionManager(handlers.HandleLeave)
	EnsureRoom := func(handler ws.EventHandler) ws.EventHandler {
		return func(c *ws.Client, m ws.MessageIncoming) {
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

		var serializer ws.Serializer
		if isHtmx {
			serializer = htmxSerializer
		} else {
			serializer = jsonSerializer
		}
		client := ws.NewClient(conn, manager, serializer)
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

	addr := fmt.Sprintf(":%s", os.Getenv("PORT"))
	log.Printf("Starting server on http://localhost%s\n", addr)
	log.Println(http.ListenAndServe(addr, r))
}
