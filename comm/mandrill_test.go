package comm_test

import (
	"log"
	"testing"

	. "github.com/hiroapp-com/hync/comm"
)

type testRecipient string

func (rcpt testRecipient) Addr() (string, string) {
	if rcpt == "" {
		return "test@hiroapp.com", "email"
	}
	return string(rcpt), "email"
}

func (rcpt testRecipient) DisplayName() string {
	return "test0r testossa"
}

func TestEmptyKey(t *testing.T) {
	MandrillKey = ""
	m := NewMandrill()
	assert(t, m == nil, "nil value expected if key is empty")
}

func TestPing(t *testing.T) {
	withMandrill(t, func(handler Handler) {
		err := handler(NewRequest("ping", testRecipient(""), map[string]interface{}{"email": "test@hiroapp.com"}))
		assert(t, err == nil, "ping failed: %s", err)
	})
}
func TestVerify(t *testing.T) {
	withMandrill(t, func(handler Handler) {
		err := handler(NewRequest("verify", testRecipient(""), map[string]interface{}{"email": "test@hiroapp.com", "token": "test"}))
		assert(t, err == nil, "verify template failed failed: %s", err)
	})
}

func TestInvite(t *testing.T) {
	withMandrill(t, func(handler Handler) {
		err := handler(NewRequest("invite", testRecipient(""), map[string]interface{}{
			"token":        "test",
			"nid":          "nid:test",
			"inviter_name": "Gargamel",
			"note_title":   "test",
			"note_peek":    "test",
		}))
		assert(t, err == nil, "invite template failed: %s", err)
	})
}
func TestInviteReject(t *testing.T) {
	withMandrill(t, func(handler Handler) {
		err := handler(NewRequest("invite", testRecipient("reject@test.mandrillapp.com"), map[string]interface{}{
			"token":        "test",
			"nid":          "nid:test",
			"inviter_name": "Gargamel",
			"note_title":   "test",
			"note_peek":    "test",
		}))
		log.Println("ERROR", err, "JUPJUP")
		assert(t, err != nil, "invite of reject-email did not fail: %s", err)
	})
}

func withMandrill(t *testing.T, fn func(Handler)) {
	//MandrillKey = "wgejxV2ulBcZrSM09cfo5g"
	MandrillKey = "k_i0T0NLtPy7k4H8PlFfyQ"
	m := NewMandrill()
	if assert(t, m != nil, "mandrill instance should be non-nill") {
		fn(m)
	}
}

func assert(t *testing.T, cond bool, msg string, args ...interface{}) bool {
	if !cond {
		t.Errorf(msg, args...)
		return false
	}
	return true
}

func assertFatal(t *testing.T, cond bool, msg string) {
	if !cond {
		t.Fatal(msg)
	}
}
