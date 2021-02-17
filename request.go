package bakuteh

import (
	"sync"
)

type Request struct {
	Cli         *Client
	Msg         *Message
	tmpSession  map[string]interface{}
	sessionLock sync.RWMutex
}

func (req *Request) Response(msg Message) {
	req.Cli.Send(&msg)

}

func (req *Request) SessionSet(key string, val interface{}) {
	req.sessionLock.Lock()
	defer req.sessionLock.Unlock()
	req.tmpSession[key] = val
}

func (req *Request) SessionGet(key string) interface{} {
	req.sessionLock.RLock()
	defer req.sessionLock.RUnlock()
	if val, ok := req.tmpSession[key]; ok {
		return val
	}

	req.Cli.sessionLock.RLock()
	defer req.Cli.sessionLock.RUnlock()
	if val, ok := req.Cli.session[key]; ok {
		return val
	} else {
		return nil
	}
}
