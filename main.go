package main

import (
	"fmt"
	"log"

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
			event, err := conn.MsgToEvent(msg)
			if err != nil {
				log.Println("invalid Message received", err)
				return
			}
			ch <- event
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
			if err = ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Println("error writing to websocket connection:", err)
				//shut. down. everything.
				return
			}
		}
	}
}

var tmpNotes = map[string]ds.Note{
	"aaaaa": ds.NewNote("a b c d e f"),
	"bbbbb": ds.NewNote("-=-=-=-=-=-"),
	"ccccc": ds.NewNote("Test"),
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
		UserID: "sk80Ms",
		Resources: []ds.Resource{
			ds.NewResource("folio", "sk80Ms"),
			//ds.NewResource("contacts", "sk80Ms"),
			ds.NewResource("note", "aaaaa"),
			ds.NewResource("note", "bbbbb"),
			ds.NewResource("note", "ccccc"),
			//ds.NewResource("meta", "aaaaa"),
			//ds.NewResource("meta", "bbbbb"),
			//ds.NewResource("meta", "ccccc"),
		},
	},
}

//User: ds.UserInfo{
//	UID:   "sk80Ms",
//	Email: "marvin@hiroapp.com",
//	Name:  "Marvin",
//},
//Settings: ds.Settings{
//	Plan: "free",
//},

var tmpFolio = map[string]ds.ResourceValue{
	"sk80Ms": ds.Folio{
		ds.NoteRef{NID: "aaaaa", Status: "active"},
		ds.NoteRef{NID: "bbbbb", Status: "active"},
		ds.NoteRef{NID: "ccccc", Status: "archive"},
	},
}

func main() {
	notify := make(ds.NotifyListener, 250)
	note_backend := ds.NewNoteMemBackend(tmpNotes)
	folioBackend := ds.NewMemBackend(func() ds.ResourceValue { return ds.Folio{} })
	folioBackend.Dict = tmpFolio
	stores := map[string]*ds.Store{
		"note":  ds.NewStore("note", note_backend, notify),
		"folio": ds.NewStore("folio", folioBackend, notify),
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
