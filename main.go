package main

import (
	"fmt"
	"log"
	"time"

	"html/template"
	"net/http"

	"github.com/gorilla/websocket"
	ds "github.com/hiro/diffsync"
)

var (
	_             = fmt.Print
	sessionHub    *ds.SessionHub
	tokenConsumer *ds.HiroTokens
)

func testHandler(c http.ResponseWriter, req *http.Request) {
	clientTempl := template.Must(template.ParseFiles("./html/client.html"))
	clientTempl.Execute(c, nil)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("handling ws request")
	// TODO: check origin and other WS best-practices
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "websocket handshake failed", 400)
		return
	} else if err != nil {
		return
	}
	defer ws.Close()
	log.Println("ping")
	conn := ds.NewConn(sessionHub.Inbox(), tokenConsumer, ds.NewJsonAdapter())
	defer conn.Close()

	from_client := make(chan ds.Event)
	// handle goroutine for message from client
	go func(ch chan ds.Event) {
		defer close(ch)
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				log.Println("error reading from websocket connection", err)
				return
			}
			msgs, err := conn.Demux(msg)
			if err != nil {
				log.Println("error de-muxing message list from client")
				continue
			}
			for i := range msgs {
				event, err := conn.MsgToEvent(msgs[i])
				if err != nil {
					log.Println("invalid Message received", err)
					return
				}
				ch <- event
			}
		}
	}(from_client)
	for {
		select {
		case event, ok := <-from_client:
			if !ok {
				// ws read failed, shutting down connection
				return
			}
			log.Println("ws: received ", event)
			conn.ClientEvent(event)
		case event, ok := <-conn.ToClient():
			if !ok {
				log.Println("error receiving from client, shutting down", err)
				//shut. down. everything.
				return
			}
			msg, err := conn.EventToMsg(event)
			if err != nil {
				log.Println("received invalid event from system", err)
				//shut. down. everything.
				return
			}
			muxed, err := conn.Mux([][]byte{msg})
			if err != nil {
				log.Println("could not mux outgoing messages into message-list", err)
				continue
			}
			if err = ws.WriteMessage(websocket.TextMessage, muxed); err != nil {
				log.Println("error writing to websocket connection:", err)
				//shut. down. everything.
				return
			}
		}
	}
}

func nao() *time.Time {
	t := time.Now()
	return &t
}

var tmpNotes = map[string]ds.Note{
	"aaaaa": ds.Note{
		Text:      ds.TextValue("a b c d e f"),
		CreatedAt: time.Now(),
		Peers: []ds.Peer{
			{User: ds.User{UID: "Flo012"}, CursorPosition: 23, LastSeen: nao(), LastEdit: nao(), Role: "owner"},
			{User: ds.User{UID: "Bruno0"}, CursorPosition: 42, LastSeen: nao(), LastEdit: nao(), Role: "active"},
			{User: ds.User{UID: "Sam012"}, CursorPosition: 0, Role: "invited"},
		},
	},
	"bbbbb": ds.Note{
		Text:      ds.TextValue("-=-=-=-=-=-"),
		CreatedAt: time.Now(),
		Peers: []ds.Peer{
			{User: ds.User{UID: "Bruno0"}, CursorPosition: 42, LastSeen: nao(), LastEdit: nao(), Role: "owner"},
			{User: ds.User{UID: "Flo012"}, CursorPosition: 23, LastSeen: nao(), LastEdit: nao(), Role: "active"},
		},
	},
	"ccccc": ds.Note{
		Text:      ds.TextValue("Test"),
		CreatedAt: time.Now(),
		Peers: []ds.Peer{
			{User: ds.User{UID: "Bruno0"}, CursorPosition: 42, LastSeen: nao(), LastEdit: nao(), Role: "owner"},
			{User: ds.User{UID: "Flo012"}, CursorPosition: 23, LastSeen: nao(), LastEdit: nao(), Role: "active"},
		},
	},
}

var tmpTokens = map[string]ds.Token{
	"anon": {
		Key:    "anon",
		UserID: "",
		Resources: []ds.Resource{
			ds.NewResource("note", "aaaaa"),
			//ds.NewResource("meta", "ak8Sk")
		},
	},
	"userlogin": {
		Key:    "userlogin",
		UserID: "Flo012",
		Resources: []ds.Resource{
			ds.NewResource("folio", "Flo012"),
			ds.NewResource("note", "aaaaa"),
			ds.NewResource("note", "bbbbb"),
			ds.NewResource("note", "ccccc"),
		},
	},
	"brunologin": {
		Key:    "brunologin",
		UserID: "Bruno0",
		Resources: []ds.Resource{
			ds.NewResource("folio", "Bruno0"),
			ds.NewResource("note", "aaaaa"),
			ds.NewResource("note", "bbbbb"),
			ds.NewResource("note", "ccccc"),
		},
	},
}

var tmpProfile = map[string]ds.ResourceValue{
	"Flo012": ds.Profile{
		User:     ds.User{UID: "Flo012", Name: "Flo", Email: "flo@qatfy.at"},
		Contacts: []ds.User{{UID: "Bruno0", Name: "Bruno"}, {UID: "Sam012", Name: "Sam"}},
	},
	"Bruno0": ds.Profile{
		User:     ds.User{UID: "Bruno0", Name: "Bruno", Email: "bruno.haid@gmail.com"},
		Contacts: []ds.User{{UID: "Flo012", Name: "Flo"}},
	},
	"Sam012": ds.Profile{
		User:     ds.User{UID: "Sam012", Name: "Sam", Email: "samaltman@ycombinator.com", Phone: "(850) 234 3241"},
		Contacts: []ds.User{{UID: "Flo012", Name: "Flo"}},
	},
}

var tmpFolio = map[string]ds.ResourceValue{
	"Flo012": ds.Folio{
		ds.NoteRef{NID: "aaaaa", Status: "active"},
		ds.NoteRef{NID: "bbbbb", Status: "active"},
		ds.NoteRef{NID: "ccccc", Status: "archive"},
	},
	"Bruno0": ds.Folio{
		ds.NoteRef{NID: "aaaaa", Status: "active"},
		ds.NoteRef{NID: "bbbbb", Status: "active"},
		ds.NoteRef{NID: "ccccc", Status: "active"},
	},
}

func main() {
	notify := make(ds.NotifyListener, 250)
	note_backend := ds.NewNoteMemBackend(tmpNotes)
	folioBackend := ds.NewMemBackend(func() ds.ResourceValue { return ds.Folio{} })
	folioBackend.Dict = tmpFolio

	profileBackend := ds.NewMemBackend(func() ds.ResourceValue { return ds.Profile{} })
	profileBackend.Dict = tmpProfile
	stores := map[string]*ds.Store{
		"note":    ds.NewStore("note", note_backend, notify),
		"folio":   ds.NewStore("folio", folioBackend, notify),
		"profile": ds.NewStore("profile", profileBackend, notify),
	}
	sess_backend := ds.NewHiroMemSessions(stores)
	tokenConsumer = ds.NewHiroTokens(sess_backend, stores)
	tokenConsumer.Tokens = tmpTokens
	sessionHub = ds.NewSessionHub(sess_backend, stores)
	go sessionHub.Run()
	go notify.Run(sess_backend, sessionHub.Inbox())

	http.HandleFunc("/client", testHandler)
	http.HandleFunc("/0/ws", wsHandler)

	log.Println("starting up http/WebSocket module")
	log.Fatal(http.ListenAndServe("localhost:8888", nil))
}
