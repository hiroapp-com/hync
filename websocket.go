package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"bitbucket.org/sushimako/diffsync"
	"github.com/gorilla/websocket"
)

var (
	adapter         = diffsync.NewJsonAdapter()
	defaultUpgrader = websocket.Upgrader{
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
)

type WsHandler struct {
	srv  *diffsync.Server
	wg   sync.WaitGroup
	done chan struct{}
	websocket.Upgrader
}

func NewWsHandler(s *diffsync.Server) *WsHandler {
	return &WsHandler{
		Upgrader: defaultUpgrader,
		srv:      s,
		done:     make(chan struct{}),
	}
}

func (h *WsHandler) Stop() {
	log.Println("ws: shutting down connections")
	close(h.done)
	h.wg.Wait()
	log.Println("ws: stopped")
}

func (h *WsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("ws: incoming connection")
	// TODO: check origin and other WS best-practices
	conn, err := h.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		log.Println("websocket: handshake failed", err)
		return
	} else if err != nil {
		return
	}
	h.wg.Add(1)
	defer h.wg.Done()
	defer func(c *websocket.Conn) {
		if err := c.WriteControl(websocket.CloseMessage, []byte{}, time.Time{}); err != nil {
			log.Println("ws: error sending websocket.CloseMessage", err)
		}
		c.Close()
	}(conn)

	from_client := make(chan diffsync.Event)
	to_client := make(chan diffsync.Event, 16)
	// inject only Client into Context passed down to server
	ctx := diffsync.Context{
		Client: diffsync.FuncHandler{func(event diffsync.Event) error {
			select {
			case to_client <- event:
				return nil
			case <-time.After(3 * time.Second):
				return diffsync.EventTimeoutError{}
			}
		}}}

	// fetch messages from WebSocket and pipe the into incoming pipe
	go func(ch chan diffsync.Event) {
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
			if err := h.srv.Handle(event); err != nil {
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
		case <-time.After(30 * time.Second):
			// heartbeat ping message evey 30s
			// currently nginx is proxying between the client and hync's
			// websocket handler. nginx has a defined proxy timeout of 60s
			// after which it will close the connection to the proxy but not
			// to the client.
			// This hopefully tells nginx that hync's listener is still alive!
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Time{}); err != nil {
				log.Println("ws: error sending websocket.PingMessage")
				return
			}
		case <-h.done:
			// shutdown
			return
		}
	}
}
