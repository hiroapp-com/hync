package comm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const SMSFrom = "+15005550006"
const TwilioURL = "https://api.twilio.com/2010-04-01/Accounts/%s/SMS/Messages.json"

var (
	TwilioSID, TwilioToken string
)

type TwilioError struct {
	info map[string]interface{}
}

func (err TwilioError) Error() string {
	return fmt.Sprintf("(twilio request failed %s)", err.info)
}

func SendSMS(to, body string) error {
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", SMSFrom)
	data.Set("Body", body)
	post, err := http.NewRequest("POST", fmt.Sprintf(TwilioURL, TwilioSID), bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	post.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	post.SetBasicAuth(TwilioSID, TwilioToken)
	resp, err := http.DefaultClient.Do(post)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp.Status, "20") {
		info := map[string]interface{}{}
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return err
		}
		return TwilioError{info}
	}
	log.Printf("(twilio sent sms to %s with body '%s')", to, body)
	return nil
}

func NewTwilio() Handler {
	if TwilioSID == "" || TwilioToken == "" {
		return nil
	}
	return func(req Request) error {
		phone, addrKind := req.Rcpt.Addr()
		if addrKind != "phone" {
			// ignore
			return nil
		}
		var body string
		switch req.Kind {
		case "invite":
			body = fmt.Sprintf("%s wants to write with you at http://localhost:5000/#%s - Hiro, writing notes together", req.Data["inviter"], req.Data["token"])
		case "verify":
			body = fmt.Sprintf("Please verify your phone-number by opening http://localhost:5000/#%s")
		default:
		}
		if body == "" {
			return fmt.Errorf("invalid phone-request kind %s", req.Kind)
		}
		return SendSMS(phone, body)
	}
}

func init() {
	TwilioSID = os.Getenv("TWILIO_SID")
	TwilioToken = os.Getenv("TWILIO_TOKEN")
}