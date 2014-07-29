package comm

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"encoding/json"
)

const MandrillAPIUrl = "https://mandrillapp.com/api/1.0"

var MandrillKey string

type MsgResponse struct {
	ID           string `json:"_id"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	RejectReason string `json:"reject_reason"`
}
type PingResponse string

type MandrillError struct {
	Status  string `json:"status"`
	Code    int64  `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (err MandrillError) Error() string {
	return fmt.Sprintf("error: %#v", err)
}

type RejectedErr struct {
	resp MsgResponse
}
type InvalidErr struct {
	resp MsgResponse
}

func (err RejectedErr) Error() string {
	return fmt.Sprintf("email to `%s` rejected; reason: `%s`; mandrill-msg-id: `%s`", err.resp.Email, err.resp.RejectReason, err.resp.ID)
}

func (err InvalidErr) Error() string {
	return fmt.Sprintf("invalid request, reason: %v", err.resp)
}

type Var struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type RcptVars struct {
	Rcpt string `json:"rcpt"`
	Vars []Var  `json:"vars"`
}

type RcptMeta struct {
	Rcpt   string                 `json:"rcpt"`
	Values map[string]interface{} `json:"values"`
}

type Message struct {
	Subject           string      `json:"subject"`
	To                []Recipient `json:"to"`
	Important         bool        `json:"important"`
	TrackOpens        bool        `json:"track_opens"`
	Merge             bool        `json:"merge"`
	MergeVars         []RcptVars  `json:"merge_vars"`
	Tags              []string    `json:"tags"`
	RecipientMetadata []RcptMeta  `json:"recipient_metadata"`
}

type MessageRequest struct {
	Key     string `json:"key"`
	Message `json:"message"`
	Async   bool `json:"async"`
}

type TemplateRequest struct {
	Key     string `json:"key"`
	Name    string `json:"template_name"`
	Content []Var  `json:"template_content"`
	Message `json:"message"`
	Async   bool `json:"async"`
}

func (tpl *TemplateRequest) AddContent(k, v string) {
	tpl.Content = append(tpl.Content, Var{Name: k, Content: v})
}

func (msg *Message) SetRcptMeta(rcpt string, meta map[string]interface{}) {
	for i := range msg.RecipientMetadata {
		if msg.RecipientMetadata[i].Rcpt == rcpt {
			msg.RecipientMetadata = append(msg.RecipientMetadata[:i], msg.RecipientMetadata[i+1:]...)
			break
		}
	}
	msg.RecipientMetadata = append(msg.RecipientMetadata, RcptMeta{Rcpt: rcpt, Values: meta})
}

func (msg *Message) SetMergeVars(rcpt string, vars map[string]string) {
	// clear old if exists
	for i := range msg.MergeVars {
		if msg.MergeVars[i].Rcpt == rcpt {
			msg.MergeVars = append(msg.MergeVars[:i], msg.MergeVars[i+1:]...)
			break
		}
	}
	msg.Merge = true
	mergeVars := make([]Var, len(vars))
	for k, v := range vars {
		mergeVars = append(mergeVars, Var{Name: k, Content: v})
	}
	msg.MergeVars = append(msg.MergeVars, RcptVars{Rcpt: rcpt, Vars: mergeVars})
}

func sendMessage(req interface{}) error {
	endpoint := ""
	switch req.(type) {
	case TemplateRequest:
		endpoint = "/messages/send-template.json"
	case MessageRequest:
		endpoint = "/messages/send-message.json"
	default:
		return fmt.Errorf("sendMessage received unknown request: %s", req)

	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	log.Println("sending req to mandrill api: ", string(data))
	resp, err := http.Post(MandrillAPIUrl+endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	// uncomment the following if you want to have the response body logged
	//	buf := make([]byte, 1024)
	//	resp.Body.Read(buf)
	//	log.Printf("received response: %#v", string(buf))
	//	return nil
	defer resp.Body.Close()
	body := json.NewDecoder(resp.Body)
	switch resp.StatusCode {
	case 200:
		log.Println("200")
		mresp := []MsgResponse{}
		if err := body.Decode(&mresp); err != nil {
			return err
		}
		switch mresp[0].Status {
		case "rejected":
			return RejectedErr{mresp[0]}
		case "invalid":
			return InvalidErr{mresp[0]}
		default:
		}
		return nil
	default:
		merr := MandrillError{}
		if err := body.Decode(&merr); err != nil {
			log.Println("cannot decode ", err)
			return err
		}
		return merr
	}
	return nil

}

func NewMandrill() func(Request) error {
	if MandrillKey == "" {
		return nil
	}
	return func(req Request) error {
		log.Println("mandrill: received request", req)
		email, addrKind := req.Rcpt.Addr()
		if addrKind != "email" {
			// ignore
			return nil
		}
		msg := NewMessageTo(email, req.Rcpt.DisplayName())
		switch req.Kind {
		case "ping":
			if err := pingAPI(MandrillKey); err != nil {
				return err
			}
		case "verify":
			msg.SetMergeVars(email, map[string]string{"TOKEN": req.Data["token"]})
			tpl := NewTemplateRequest("verify", msg)
			err := sendMessage(tpl)
			switch err.(type) {
			case RejectedErr:
				// the caller should inspect the reason for rejection. if it's a
				// hard-bounce, we might wanna abort the signup process with an
				// error? if it is 'spam', we should ask the user to check his
				// spam-folder (need to figured out if spam-rejected msgs
				// are actually received by the other mx or not)
				return err
			case InvalidErr:
				// log and continue like nothing happened. we are just loosing the feedback here
				// but facing the user we will accept the email
				log.Printf("error: mandrill reported invalid request. request: %s, err: %s", tpl, err)
			case MandrillError:
				return err
			case nil:
			}
			// all done
		case "invite":
			msg.SetMergeVars(email, map[string]string{
				"TOKEN":        req.Data["token"],
				"NOTE_ID":      req.Data["nid"],
				"INVITER_NAME": req.Data["inviter_name"],
			})
			tpl := NewTemplateRequest("invite", msg)
			tpl.AddContent("note_title", req.Data["note_title"])
			tpl.AddContent("note_peek", req.Data["note_peek"])
			if numPeers, _ := strconv.Atoi(req.Data["num_peers"]); numPeers > 2 {
				tpl.AddContent("extra_peers", fmt.Sprintf("(and %d other people)", numPeers-2))
			}
			err := sendMessage(tpl)
			switch err.(type) {
			case RejectedErr:
				// caller should inspect the reason for rejection.
				// if it's 'spam', the email should be marked as such
				// (e.g. email_status="blocked"? TBD) to reduce the risk of
				// spammers on our plattform
				return err
			case InvalidErr:
				// log and continue like nothing happened. we are just loosing the feedback here
				// but facing the user we will accept the email
				log.Printf("error: mandrill reported invalid request. request: %s, err: %s", req, err)
			case nil:
			default:
				return err
			}
			// all done
		}
		return nil
	}
}

func NewMessageTo(email, name string) Message {
	return Message{
		To:                []Recipient{Recipient{Email: email, Name: name, Type: "to"}},
		Important:         false,
		TrackOpens:        true,
		MergeVars:         []RcptVars{},
		Tags:              []string{}, //XXX
		RecipientMetadata: []RcptMeta{},
	}
}

func NewTemplateRequest(name string, msg Message) TemplateRequest {
	return TemplateRequest{
		Key:     MandrillKey,
		Name:    name,
		Content: []Var{},
		Message: msg,
	}
}

func pingAPI(key string) error {
	data, err := json.Marshal(struct {
		Key string `json:"key"`
	}{key})
	if err != nil {
		return err
	}
	log.Println("sending ping to mandrill api: ", string(data))
	resp, err := http.Post(MandrillAPIUrl+"/users/ping.json", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
		return nil
	default:
		merr := MandrillError{}
		if err := json.NewDecoder(resp.Body).Decode(&merr); err != nil {
			return err
		}
		return merr
	}
}

func init() {
	MandrillKey = os.Getenv("MANDRILL_KEY")
	if MandrillKey == "" {
		return
	}
	log.Println("testing mandrill api key...")
	err := pingAPI(MandrillKey)
	if err != nil {
		panic(err)
	}
	log.Println("mandrill o.k.")

}
