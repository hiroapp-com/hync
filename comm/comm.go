package comm

import "log"

type Rcpt interface {
	DisplayName() string
	Addr() (string, string)
}

type Handler func(Request) error

type RequestTimeoutError struct{}

type Request struct {
	Kind string
	Rcpt
	Data map[string]string
}

type StaticRcpt struct {
	name string
	addr string
	kind string
}

func NewRequest(kind string, rcpt Rcpt, data map[string]string) Request {
	return Request{
		Kind: kind,
		Rcpt: rcpt,
		Data: data,
	}
}

func (rcpt StaticRcpt) DisplayName() string {
	return rcpt.name
}
func (rcpt StaticRcpt) Addr() (string, string) {
	return rcpt.addr, rcpt.kind
}

func (err RequestTimeoutError) Error() string {
	return "the communication request has timed out"
}

func NewStaticRcpt(name, addr, kind string) StaticRcpt {
	return StaticRcpt{name: name, addr: addr, kind: kind}
}

func NewLogHandler() func(Request) error {
	return func(req Request) error {
		log.Println("comm: received request: ", req)
		return nil
	}
}
