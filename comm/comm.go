package comm

import (
	"encoding/json"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
)

type Rcpt interface {
	DisplayName() string
	Addr() (string, string)
}

type Handler func(Request) error

func HandlerGroup(fns ...Handler) Handler {
	// log errors
	errch := make(chan error)
	go func(ch chan error) {
		for err := range ch {
			if err != nil {
				log.Println("error while processing comm.Request: ", err)
			}
		}
	}(errch)
	return func(req Request) error {
		for i := range fns {
			go func(fn Handler) {
				errch <- fn(req)
			}(fns[i])
		}
		return nil
	}
}

type RequestTimeoutError struct{}

type Request struct {
	Kind string
	Rcpt
	Data map[string]interface{}
}

type StaticRcpt struct {
	Name    string `json:"name"`
	Address string `json:"addr"`
	Kind    string `json:"kind"`
}

func NewRequest(kind string, rcpt Rcpt, data map[string]interface{}) Request {
	return Request{
		Kind: kind,
		Rcpt: rcpt,
		Data: data,
	}
}

func (rcpt StaticRcpt) DisplayName() string {
	return rcpt.Name
}
func (rcpt StaticRcpt) Addr() (string, string) {
	return rcpt.Address, rcpt.Kind
}

func (err RequestTimeoutError) Error() string {
	return "the communication request has timed out"
}

func NewStaticRcpt(name, addr, kind string) StaticRcpt {
	return StaticRcpt{Name: name, Address: addr, Kind: kind}
}

func NewLogHandler() func(Request) error {
	return func(req Request) error {
		log.Println("comm: received request: ", req)
		return nil
	}
}

func (req *Request) UnmarshalJSON(src []byte) error {
	tmp := struct {
		Kind string     `json:"kind"`
		Rcpt StaticRcpt `json:"rcpt"`
		Data map[string]interface{}
	}{
		Rcpt: StaticRcpt{},
		Data: map[string]interface{}{},
	}
	if err := json.Unmarshal(src, &tmp); err != nil {
		return err
	}
	req.Kind = tmp.Kind
	req.Rcpt = tmp.Rcpt
	req.Data = tmp.Data
	return nil
}

type WrapRPC Handler

func (wrapped WrapRPC) Send(req Request, errStr *string) error {
	handler := Handler(wrapped)
	if err := handler(req); err != nil {
		return err
	}
	return nil
}
func (wrapped WrapRPC) Run(l net.Listener) {
	log.Printf("running RPC-Wrapped comm.Handler at %s", l.Addr())
	rpc.Register(wrapped)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("rpc: accept err; %s", err)
			continue
		}
		log.Printf("rpc-conn-handler: new client connection established: %s", conn.RemoteAddr())
		go jsonrpc.ServeConn(conn)
	}
}
