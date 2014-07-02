package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"html/template"
	"net/http"
	"runtime/pprof"

	"github.com/gorilla/websocket"
	ds "github.com/hiro/diffsync"
	_ "github.com/mattn/go-sqlite3"
)

const (
	HYNC_VERSION  = "0.6"
	HYNC_CODENAME = "e043e10440"
)

var (
	_             = fmt.Print
	sessionHub    *ds.SessionHub
	tokenConsumer *ds.HiroTokens
	store         *ds.Store
)
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var listenAddr = flag.String("listen", "0.0.0.0:8888", "listen on socket")

func testHandler(c http.ResponseWriter, req *http.Request) {
	clientTempl := template.Must(template.ParseFiles("./html/client.html"))
	clientTempl.Execute(c, nil)
}

func generateToken() string {
	uuid := make([]byte, 16)
	if n, err := rand.Read(uuid); err != nil || n != len(uuid) {
		panic(err)
	}
	// RFC 4122
	uuid[8] = 0x80 // variant bits
	uuid[4] = 0x40 // v4
	return hex.EncodeToString(uuid)
}

func anonTokenHandler(db *sql.DB) http.HandlerFunc {
	return func(c http.ResponseWriter, req *http.Request) {
		anonToken := generateToken()
		_, err := db.Exec("INSERT INTO tokens (token, uid, nid) VALUES (?, '', '')", anonToken)
		if err != nil {
			c.Write([]byte(err.Error()))
			return
		}
		log.Println("tokens: created anon token: ", anonToken)
		c.Write([]byte(anonToken))
	}
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
	conn := ds.NewConn(sessionHub.Inbox(), tokenConsumer, ds.NewJsonAdapter(), store)
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
					// N.B. returning here means we're shutting the whole connection
					// down in the event of a malformed incoming message.
					// This might be rather drastic behaviour, but for now i'll keep
					// it in so bug due to malformed data will die severly. in the
					// hope to keep those bugs out for the release
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

func nao() *ds.UnixTime {
	t := ds.UnixTime(time.Now())
	return &t
}
func main() {
	log.Println("Spinning up the Hync.")
	log.Printf("  > version `%s`\n", HYNC_VERSION)
	log.Printf("  > codename `%s`\n\n", HYNC_CODENAME)
	flag.Parse()

	if *cpuprofile != "" {
		// start profiler
		// NOTE: does not really work, yet. first we need a graceful shutdown
		//       of the websever
		log.Printf("CPU profile requested")
		prof, _ := os.Create(*cpuprofile)
		defer prof.Close()

		pprof.StartCPUProfile(prof)
		defer pprof.StopCPUProfile()
	}

	// connect to DB
	db, err := sql.Open("sqlite3", "./hiro.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	notify := make(ds.NotifyListener, 250)
	store = ds.NewStore(db, notify)
	store.Mount("note", ds.NewNoteSQLBackend(db))
	store.Mount("folio", ds.NewFolioSQLBackend(db))
	store.Mount("profile", ds.NewProfileSQLBackend(db))

	sessionBackend := ds.NewSQLSessions(db)
	tokenConsumer = ds.NewHiroTokens(sessionBackend, db)
	sessionHub = ds.NewSessionHub(sessionBackend)
	go sessionHub.Run()
	go notify.Run(sessionBackend, sessionHub.Inbox())

	http.HandleFunc("/anontoken", anonTokenHandler(db))
	http.HandleFunc("/client", testHandler)
	http.HandleFunc("/0/ws", wsHandler)

	log.Println("starting up http/WebSocket module")
	log.Printf("listening on http://%s\n", *listenAddr)
	log.Println(http.ListenAndServe(*listenAddr, nil))
}
