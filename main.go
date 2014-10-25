package main

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"database/sql"
	"html/template"
	"net"

	"net/http"
	"runtime/pprof"

	"github.com/gorilla/websocket"
	ds "github.com/hiro/diffsync"
	"github.com/hiro/hync/comm"
	_ "github.com/lib/pq"
)

const (
	HYNC_VERSION  = "0.7"
	HYNC_CODENAME = "HollyHug"
)

var (
	_              = fmt.Print
	DefaultAdapter = ds.NewJsonAdapter()
	srv            *ds.Server
	cpuprofile     = flag.String("cpuprofile", "", "write cpu profile to file")
	listenAddr     = flag.String("listen", "0.0.0.0:8888", "listen on socket")
	commListenAddr = flag.String("conn_listen", "0.0.0.0:7777", "listen JSON-RPC server for communication handling on this addr")
	dbHost         = flag.String("db_host", "postgres://hiro:hiro@localhost:5432/hiro?sslmode=require", "connection string to establish PgSQL connection")
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	Subprotocols:     []string{"hync"},
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin: func(r *http.Request) bool {
		switch r.Header.Get("Origin") {
		case "http://localhost:5000", "https://beta.hiroapp.com":
		default:
			return false
		}
		return true
	},
}

func testHandler(c http.ResponseWriter, req *http.Request) {
	clientTempl := template.Must(template.ParseFiles("./html/client.html"))
	clientTempl.Execute(c, nil)
}

func anonTokenHandler(db *sql.DB) http.HandlerFunc {
	return func(c http.ResponseWriter, req *http.Request) {
		token, err := srv.Token("anon")
		if err != nil {
			log.Println("failed at creating anon token: ", err)
			return
		}
		log.Println("tokens: created anon token: ", token)
		c.Write([]byte(token))
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("ws: incoming connection")
	// TODO: check origin and other WS best-practices
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		log.Println("websocket: handshake failed", err)
		return
	} else if err != nil {
		return
	}
	defer conn.Close()
	adapter := DefaultAdapter

	from_client := make(chan ds.Event)
	to_client := make(chan ds.Event, 16)
	// inject only Client into Context passed down to server
	ctx := ds.Context{
		Client: ds.FuncHandler{func(event ds.Event) error {
			select {
			case to_client <- event:
				return nil
			case <-time.After(3 * time.Second):
				return ds.EventTimeoutError{}
			}
		}}}

	// fetch messages from WebSocket and pipe the into incoming pipe
	go func(ch chan ds.Event) {
		defer close(ch)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("error reading from websocket connection", err)
				return
			}
			msgs, err := adapter.Demux(msg)
			if err != nil {
				log.Println("error de-muxing message list from client")
				continue
			}
			for i := range msgs {
				event, err := adapter.MsgToEvent(msgs[i])
				if err != nil {
					log.Println("invalid Message received", err)
					// N.B. returning here means we're shutting the whole connection
					// down in the event of a malformed incoming message.
					// This might be rather drastic behaviour, but for now i'll keep
					// it in so bug due to malformed data will die severly. in the
					// hope to keep those bugs out for the release
					return
				}
				event.Context(ctx)
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
			if err := srv.Handle(event); err != nil {
				log.Println("websocket: server could not handle incoming event", err)
			}
		case event, ok := <-to_client:
			if !ok {
				log.Println("error receiving from client, shutting down", err)
				//shut. down. everything.
				return
			}
			msg, err := adapter.EventToMsg(event)
			if err != nil {
				log.Println("received invalid event from system", err)
				//shut. down. everything.
				return
			}
			muxed, err := adapter.Mux([][]byte{msg})
			if err != nil {
				log.Println("could not mux outgoing messages into message-list", err)
				continue
			}
			if err = conn.WriteMessage(websocket.TextMessage, muxed); err != nil {
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

func commRPCServer(handler comm.Handler, addr string) {
	// start RPC wrapper for comm.Handler
	commListener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer commListener.Close()
	commRPC := comm.WrapRPC(handler)
	commRPC.Run(commListener)
}

func main() {
	flag.Parse()
	log.Println("Spinning up the Hync.")
	log.Printf("  > version `%s`\n", HYNC_VERSION)
	log.Printf("  > codename `%s`\n\n", HYNC_CODENAME)

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
	db, err := sql.Open("postgres", *dbHost)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	commHandlers := []comm.Handler{}
	if sendwithus := comm.NewSendwithus(); sendwithus != nil {
		commHandlers = append(commHandlers, sendwithus)
	}
	if twilio := comm.NewTwilio(); twilio != nil {
		commHandlers = append(commHandlers, twilio)
	}
	if len(commHandlers) == 0 {
		// no comm handlers configured, fallback to logger
		commHandlers = []comm.Handler{comm.NewLogHandler()}
	}
	commHandler := comm.HandlerGroup(commHandlers...)
	go commRPCServer(commHandler, *commListenAddr)

	// create server environment
	srv, err = ds.NewServer(db, commHandler)
	if err != nil {
		panic(err)
	}
	defer srv.Stop()

	srv.Store.Mount("note", ds.NewNoteSQLBackend(db))
	srv.Store.Mount("folio", ds.NewFolioSQLBackend(db))
	srv.Store.Mount("profile", ds.NewProfileSQLBackend(db))
	srv.Run()

	http.HandleFunc("/anontoken", anonTokenHandler(db))
	http.HandleFunc("/client", testHandler)
	http.HandleFunc("/0/ws", wsHandler)

	log.Println("starting up http/WebSocket module")
	log.Printf("listening on http://%s\n", *listenAddr)
	log.Println(http.ListenAndServe(*listenAddr, nil))
}

func generateToken() (string, string) {
	uuid := make([]byte, 16)
	if n, err := rand.Read(uuid); err != nil || n != len(uuid) {
		panic(err)
	}
	// RFC 4122
	uuid[8] = 0x80 // variant bits
	uuid[4] = 0x40 // v4
	plain := hex.EncodeToString(uuid)
	h := sha512.New()
	h.Write(uuid)
	hashed := hex.EncodeToString(h.Sum(nil))
	//hashed_key := fmt.Sprintf("%x", h.Sum(nil))
	log.Printf("CREATED TOKEN: uuid: `%v` plain: `%s`, hashed: `%s`", uuid, plain, hashed)
	return plain, hashed
}
