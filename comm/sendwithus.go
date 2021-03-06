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
	Data   map[string]interface{} `json:"email_data"`
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
		email, addrKind := r.Rcpt.Addr()
		if addrKind != "email" {
			// ignore
			return nil
		}
		log.Println("sendwithus: received request", r)
		// send a template
		tpl := SWUTemplateRequest{EmailID: r.Kind, Data: r.Data}
		switch r.Kind {
		case "signup-verify", "verify":
			// SWU: "Verify Email"
			tpl.EmailID = "tem_kVj6HXpeeSekDDSFFXXAqC"
		case "reset-pwd":
			// SWU: "Reset password"
			tpl.EmailID = "tem_6WSwM6oCtA2xnmCVZ52FvR"
		case "invite":
			// SWU: "Sharing"
			tpl.EmailID = "tem_o26JFZYdbr6u58whiZRd37"
		case "signup-setpwd":
			// SWU: "Generic Transactional"
			tpl.EmailID = "tem_Gk92owL3EYH9HUhjLPjtKN"
		case "notify-inactive":
			// SWU: "Generic Transactional"
			tpl.EmailID = "tem_Ft7sxJGtAfwrQNztAnPbzF"
		case "invite-accepted":
			// SWU: "Invitation Accepted"
			tpl.EmailID = "tem_DsjvFKzQZFJ9RrxBSd5Hha"
		case "welcome", "welcome-setpwd":
			// ignore until these flows are implemented
			return nil
		}
		r.Data["reason"] = r.Kind
		tpl.Data = r.Data
		tpl.Rcpt.Name = r.Rcpt.DisplayName()
		tpl.Rcpt.Address = email
		if e, ok := r.Data["inviter_email"]; ok {
			tpl.Sender.ReplyTo = e.(string)
		}
		data, err := json.Marshal(tpl)
		if err != nil {
			return err
		}
		//log.Println(string(data))
		res, err := swuApiRequest(SWUSendURL, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("sendwithus: http request failed: %s", err.Error())
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			m, _ := ioutil.ReadAll(res.Body)
			return fmt.Errorf("sendwithus: send request failed: %d %s", res.StatusCode, string(m))
		}
		resp := SWUResponse{}
		if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
			return fmt.Errorf("sendwithus: json error: %s", err)
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
