package main

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"database/sql"
	"html/template"
	"net"

	"net/http"
	"runtime/pprof"

	"bitbucket.org/sushimako/diffsync"
	"bitbucket.org/sushimako/hync/comm"
	_ "github.com/lib/pq"
)

const (
	HYNC_VERSION  = "0.7"
	HYNC_CODENAME = "HollyHug"
)

var (
	_              = fmt.Print
	srv            *diffsync.Server
	cpuprofile     = flag.String("cpuprofile", "", "write cpu profile to file")
	listenAddr     = flag.String("listen", "0.0.0.0:8888", "listen on socket")
	commListenAddr = flag.String("conn_listen", "0.0.0.0:7777", "listen JSON-RPC server for communication handling on this addr")
	dbHost         = flag.String("db_host", "postgres://hiro:hiro@localhost:5432/hiro?sslmode=require", "connection string to establish PgSQL connection")
)

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

func nao() *diffsync.UnixTime {
	t := diffsync.UnixTime(time.Now())
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
	srv, err = diffsync.NewServer(db, commHandler)
	if err != nil {
		panic(err)
	}
	defer srv.Stop()

	srv.Store.Mount("note", diffsync.NewNoteSQLBackend(db))
	srv.Store.Mount("folio", diffsync.NewFolioSQLBackend(db))
	srv.Store.Mount("profile", diffsync.NewProfileSQLBackend(db))
	srv.Run()

	wsh := NewWsHandler(srv)
	http.HandleFunc("/anontoken", anonTokenHandler(db))
	http.HandleFunc("/client", testHandler)
	http.Handle("/0/ws", wsh)
	defer wsh.Stop()

	log.Println("starting up http/WebSocket module")
	log.Printf("listening on http://%s\n", *listenAddr)
	go func() {
		log.Println(http.ListenAndServe(*listenAddr, nil))
	}()
	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	log.Println("signal", <-sigch)
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
