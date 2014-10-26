package comm_test

import (
	"log"
	"testing"

	. "bitbucket.org/sushimako/hync/comm"
)

const (
	validPhone   = "+15006660000"
	invalidPhone = "+15005550001"
)

func newTestTwilio() Handler {
	TwilioSID = "AC103ca98517c9bc3a0151b6dd75967970"
	TwilioToken = "f7e450364559c1c77003301652661216"
	return NewTwilio()
}

func TestTwilioSendInvite(t *testing.T) {
	twilio := newTestTwilio()
	if assert(t, twilio != nil, "twilio handler invalid") {
		req := NewRequest("invite", NewStaticRcpt("", validPhone, "phone"), map[string]string{"inviter": "+123456", "token": "test"})
		err := twilio(req)
		assert(t, err == nil, "twilio request failed: %s", err)
	}
}

func TestTwilioFailSendInvite(t *testing.T) {
	twilio := newTestTwilio()
	if assert(t, twilio != nil, "twilio handler invalid") {
		req := NewRequest("invite", NewStaticRcpt("", invalidPhone, "phone"), map[string]string{"inviter": "+123456", "token": "test"})
		err := twilio(req)
		log.Println(err)
		assert(t, err != nil, "twilio request should have failed")
	}
}
