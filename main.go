package main

import (
	"encoding/json"
	"fmt"
	ds "github.com/hiro/diffsync"
	"log"
)

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

	token_consumer := ds.NewHiroTokens(sess_backend, stores)
    token_consumer.Tokens = tmpTokens
	sess_hub := ds.NewSessionHub(sess_backend, stores)
	go sess_hub.Run()

	// this would come from the client and reuse the same buffer again and again
	ev := ds.NewEvent("session-create", "", map[string]string{"token": "userlogin"}, nil)
	sid, err := getSessionId(ev, token_consumer)
	if err != nil {
		fmt.Println(err)
	}
	log.Println("SSIIDD", sid)
	client := make(chan ds.Event)
	newevent := ds.NewEvent("session-create", sid, nil, client)
	sess_hub.Inbox() <- newevent

	fmt.Println("waiting on client response...")
	resp := <-client
	fmt.Printf("response received: %v %v\n", resp, resp.Data())
	data, ok := resp.Data().(ds.SessionData)
	if !ok {
		panic("WHUT")
	}
	log.Printf("\n\n%v", data)
	foo, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Println("CANNOT MARSHAL SessionData:", err)
	}
	_ = foo
	fmt.Println(string(foo))
	x := ds.NewNoteValue("FOOOO")
	fmt.Println("here we go", x.String())

	changes := []ds.Edit{ds.NewEdit(ds.NoteDelta("-3\t=8"))}

	// try sending a sync request
	newevent = ds.NewEvent("res-sync", sid, ds.NewSyncData("note", "aaaaa", changes), client)
	sess_hub.Inbox() <- newevent

	fmt.Println("waiting on client response...")
	resp = <-client
	fmt.Printf("response received: %#v \n", resp)

	data2, ok := resp.Data().(ds.SyncData)
	if !ok {
		log.Println("wrong type for payload")
	} else {
		log.Println("whoops nothing to see here")
	}
	log.Printf(">>>> %#v", data2)
	foo, err = json.MarshalIndent(data2, "", "  ")
	if err != nil {
		log.Println("CANNOT MARSHAL SyncData:", err)
	}
	log.Println(string(foo))

	log.Println("notestoe-dump!")
	log.Println(note_backend.DumpAll())

	return
}

func getSessionId(event ds.Event, consumer ds.TokenConsumer) (sid string, err error) {
	if event.Name() == "session-create" {
		log.Println("received session-create, parsing tokens", event)
		data := event.Data().(map[string]string)
		sid, err = consumer.Consume(data["token"], event.SID())
		log.Printf("consumed token `%s` and received sessionid `%s`", data["token"], sid)
		return
	}
	return event.SID(), nil
}
