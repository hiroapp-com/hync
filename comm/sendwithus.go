package comm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const SWUSendURL = "https://api.sendwithus.com/api/v1/send"

var SWUApiKey string

type SWUTemplateRequest struct {
	EmailID string `json:"email_id"`
	Rcpt    struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	} `json:"recipient"`
	Data   map[string]string `json:"email_data"`
	Sender struct {
		ReplyTo string `json:"reply_to"`
	} `json:"sender"`
}

type SWUResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	// ignore all remaining repsonse info
}

func swuApiRequest(endpoint string, r io.Reader) (*http.Response, error) {
	hr, err := http.NewRequest("POST", endpoint, r)
	if err != nil {
		return nil, err
	}
	hr.SetBasicAuth(SWUApiKey, "")
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("X-SWU-API-CLIENT", "hiroapp-swu-0.1")
	return http.DefaultClient.Do(hr)
}

func NewSendwithus() func(Request) error {
	if SWUApiKey == "" {
		return nil
	}
	return func(r Request) error {
		log.Println("sendwithus: received request", r)
		email, addrKind := r.Rcpt.Addr()
		if addrKind != "email" {
			// ignore
			return nil
		}
		// send a template
		tpl := SWUTemplateRequest{EmailID: r.Kind, Data: r.Data}
		switch r.Kind {
		case "signup-verify", "signup-setpwd", "reset-pwd", "welcome", "welcome-setpwd":
			tpl.EmailID = "tem_Gk92owL3EYH9HUhjLPjtKN"
		case "invite":
			tpl.EmailID = "tem_o26JFZYdbr6u58whiZRd37"
		}
		r.Data["reason"] = r.Kind
		tpl.Data = r.Data
		tpl.Rcpt.Name = r.Rcpt.DisplayName()
		tpl.Rcpt.Address = email
		if e, ok := r.Data["inviter_email"]; ok {
			tpl.Sender.ReplyTo = e
		}
		data, err := json.Marshal(tpl)
		if err != nil {
			return err
		}
		res, err := swuApiRequest(SWUSendURL, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("sendwithus: http request failed: %d %s", res.StatusCode, err.Error())
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			m, _ := ioutil.ReadAll(res.Body)
			return fmt.Errorf("sendwithus: send request failed: %d %s", res.StatusCode, string(m))
		}
		resp := SWUResponse{}
		err = json.NewDecoder(res.Body).Decode(&resp)
		if err != nil {
			return err
		} else if !resp.Success {
			return fmt.Errorf("sendwithus: send of template NOT successfull with status `%s`", resp.Status)
		}
		return nil
	}
}

func init() {
	SWUApiKey = os.Getenv("SENDWITHUS_KEY")
	if SWUApiKey == "" {
		return
	}
}
