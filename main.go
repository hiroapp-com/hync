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
	conn := ds.NewConn(sessionHub.Inbox(), tokenConsumer)
	defer conn.Close()

	from_client := make(chan ds.Event)
	go func(ch chan ds.Event) {
		defer close(ch)
		var buf ds.Event
		for {
			buf = ds.NewEvent()
			if err := ws.ReadJSON(&buf); err != nil {
				log.Println("error reading from websocket connection", err)
				return
			}
			ch <- buf
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
			if err := ws.WriteJSON(event); err != nil {
				log.Println("error writing to websocket connection:", err)
				//shut. down. everything.
				return
			}
		}
	}
}

var tmpStore = map[string]*ds.NoteValue{
	"aaaaa": ds.NewNoteValue("a b c d e f"),
	"bbbbb": ds.NewNoteValue("-=-=-=-=-=-"),
	"ccccc": ds.NewNoteValue("Test"),
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
		UserID: "testUser",
		Resources: []ds.Resource{
			//ds.NewResource("folio", "sk80Ms"),
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

func main() {
	note_backend := ds.NewNoteMemBackend(tmpStore)
	stores := map[string]*ds.Store{
		"note": ds.NewStore(note_backend),
	}
	sess_backend := ds.NewHiroMemSessions(stores)
	tokenConsumer = ds.NewHiroTokens(sess_backend, stores)
	tokenConsumer.Tokens = tmpTokens
	sessionHub = ds.NewSessionHub(sess_backend, stores)
	go sessionHub.Run()

	http.HandleFunc("/client", testHandler)
	http.HandleFunc("/0/ws", wsHandler)

	log.Println("starting up http/WebSocket module")
	log.Fatal(http.ListenAndServe("localhost:8888", nil))
}
