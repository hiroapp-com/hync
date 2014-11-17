package comm_test

import (
	"testing"

	. "bitbucket.org/sushimako/hync/comm"
)

var testRcpt = StaticRcpt{
	Name:    "Testa Rossa",
	Address: "test@hiroapp.com",
	Kind:    "email",
}

func TestSWUInvite(t *testing.T) {
	withHandler(t, func(handler Handler) {
		err := handler(NewRequest("invite", testRcpt, map[string]interface{}{
			"token":         "test",
			"note_id":       "nid:test",
			"inviter_email": "hello@qatfy.at",
		}))
		assert(t, err == nil, "invite template failed: %s", err)
	})
}

func TestSWUVerify(t *testing.T) {
	withHandler(t, func(handler Handler) {
		err := handler(NewRequest("verify", testRcpt, map[string]interface{}{
			"token": "test",
		}))
		assert(t, err == nil, "invite template failed: %s", err)
	})
}

func TestSWUResetPW(t *testing.T) {
	withHandler(t, func(handler Handler) {
		err := handler(NewRequest("reset-pw", testRcpt, map[string]interface{}{
			"token": "test",
		}))
		assert(t, err == nil, "invite template failed: %s", err)
	})
}

func withHandler(t *testing.T, fn func(Handler)) {
	SWUApiKey = "test_76508d1e724c6b408662749aba2c51756043ef4c"
	h := NewSendwithus()
	if assert(t, h != nil, "sendwithus instance should be non-nill") {
		fn(h)
	}
}

//func assert(t *testing.T, cond bool, msg string, args ...interface{}) bool {
//	if !cond {
//		t.Errorf(msg, args...)
//		return false
//	}
//	return true
//}
