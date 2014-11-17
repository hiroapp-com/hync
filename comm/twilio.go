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

const SMSFrom = "+16506207887"
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
		log.Println("twilio: received request", req)
		var inviterName, inviterPhone, inviterEmail string
		if s, ok := req.Data["inviter_name"].(string); ok {
			inviterName = s
		}
		if s, ok := req.Data["inviter_phone"].(string); ok {
			inviterPhone = s
		}
		if s, ok := req.Data["inviter_email"].(string); ok {
			inviterEmail = s
		}
		var body string
		switch req.Kind {
		case "invite":
			from := firstNonEmpty(inviterName, inviterPhone, inviterEmail, "Someone")
			if len(from) > 25 {
				from = from[:25]
			}
			peek := firstNonEmpty(req.Data["title"].(string), req.Data["peek"].(string), "New Note")
			remaining := 160 - len(from) - 32 - 56 // update last number when changin the share-text
			if len(peek) > remaining {
				peek = peek[:remaining-3] + "..."
			}
			body = fmt.Sprintf("%s shared the note '%s' with you: https://beta.hiroapp.com/#%s", from, peek, req.Data["token"].(string))
		case "verify":
			body = fmt.Sprintf("Please verify your device by visiting https://beta.hiroapp.com/#v:%s", req.Data["token"].(string))
		case "reset-pwd":
			body = fmt.Sprintf("You can now reset your password at https://beta.hiroapp.com/#r:%s", req.Data["token"].(string))
		default:
		}
		if body == "" {
			return fmt.Errorf("invalid phone-request kind %s", req.Kind)
		}
		return SendSMS(phone, body)
	}
}

func firstNonEmpty(args ...string) string {
	for i := range args {
		if args[i] != "" {
			return args[i]
		}
	}
	return ""
}

func init() {
	TwilioSID = os.Getenv("TWILIO_SID")
	TwilioToken = os.Getenv("TWILIO_TOKEN")
}
