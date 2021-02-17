package bakuteh

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	srv         *Server
	conn        *websocket.Conn
	accepter    chan *Signal
	session     map[string]interface{}
	sessionLock sync.RWMutex
	reqQueue    []Message
	currReq     *Request
	closed      bool
}

func newClient(srv *Server, conn *websocket.Conn, session map[string]interface{}) *Client {
	cli := &Client{
		srv:      srv,
		conn:     conn,
		accepter: make(chan *Signal),
		session:  make(map[string]interface{}),
		closed:   false,
	}

	srv.accepter <- &Signal{Action: signalRegister, Cli: cli}

	if session != nil {
		for k, v := range session {
			cli.session[k] = v
		}
	}

	go cli.readPump()
	go cli.writePump()

	return cli
}

func (cli *Client) readPump() {
	defer func() {
		cli.close("", true)
	}()
	cli.conn.SetReadLimit(maxMessageSize)
	cli.conn.SetReadDeadline(time.Now().Add(pongWait))
	cli.conn.SetPongHandler(func(string) error { cli.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	if _, ok := cli.srv.actions[ACTION_INIT]; ok {
		cli.NextRequest(Message{Action: ACTION_INIT})
		cli.processRequest()
	}

	for {
		t, message, err := cli.conn.ReadMessage()
		if err != nil {
			// if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			//log.Printf("error: %v", err)
			// }
			return
		}
		fmt.Println(t, string(message))

		msg := Message{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Printf("json decode error: %s", message)
			continue
		}
		cli.NextRequest(msg)

		cli.processRequest()

	}
}

func (cli *Client) writePump() {

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		cli.close("", true)
	}()
	for {
		select {
		case sgl, ok := <-cli.accepter:
			cli.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				return
			}

			w, err := cli.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				if sgl.Err != nil {
					sgl.Err <- err
					close(sgl.Err)
				}
				return
			}

			message, _ := json.Marshal(sgl.Msg)
			w.Write(message)
			if err := w.Close(); err != nil {
				if sgl.Err != nil {
					sgl.Err <- err
					close(sgl.Err)
				}
				return
			}

			if sgl.Err != nil {
				close(sgl.Err)
			}

		case <-ticker.C:
			cli.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cli.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}

}

func (cli *Client) close(reason string, passive bool) {

	cli.sessionLock.Lock()
	if cli.closed {
		cli.sessionLock.Unlock()
		return
	} else {
		cli.closed = true
		cli.sessionLock.Unlock()
	}

	defer func() {
		close(cli.accepter)
		cli.conn.Close()
	}()

	cli.NextRequest(Message{Action: ACTION_CLOSE, Data: reason})
	cli.processRequest()

	if passive {
		cli.srv.accepter <- &Signal{Action: signalUnregisterP, Cli: cli}
	} else {
		cli.srv.accepter <- &Signal{Action: signalUnregister, Cli: cli}

		sgl := &Signal{
			Action: signalSend,
			Msg:    &Message{Action: ACTION_CLOSE, Data: reason},
			Err:    make(chan error),
		}
		cli.accepter <- sgl
		<-sgl.Err
	}
}

func (cli *Client) Send(msg *Message, chs ...chan error) {
	var ch chan error
	if len(chs) == 1 {
		ch = chs[0]
	}
	cli.sessionLock.RLock()
	if cli.closed {
		if ch != nil {
			ch <- errors.New("client close")
			close(ch)
		}
		return
	}
	cli.sessionLock.RUnlock()

	sgl := &Signal{
		Action: signalSend,
		Msg:    msg,
		Err:    ch,
	}

	cli.accepter <- sgl
}

func (cli *Client) Broadcast(msg *Message) {
	cli.srv.Broadcast(msg)
}

func (cli *Client) NextRequest(msg Message) {
	cli.sessionLock.Lock()
	defer cli.sessionLock.Unlock()
	cli.reqQueue = append(cli.reqQueue, msg)
}

func (cli *Client) processRequest() {
	cli.sessionLock.Lock()
	if len(cli.reqQueue) == 0 {
		cli.sessionLock.Unlock()
		return
	}
	defer cli.processRequest()
	msg := cli.reqQueue[0]
	if cli.closed && msg.Action != ACTION_CLOSE {
		cli.reqQueue = cli.reqQueue[1:]
		cli.sessionLock.Unlock()
		return
	}

	req := &Request{
		Cli:        cli,
		Msg:        &msg,
		tmpSession: make(map[string]interface{}),
	}

	if fn, ok := cli.srv.actions[msg.Action]; ok {
		cli.currReq = req
		cli.sessionLock.Unlock()
		fn(req)
		cli.sessionLock.Lock()
	}

	cli.currReq = nil
	cli.reqQueue = cli.reqQueue[1:]
	for k, v := range req.tmpSession {
		cli.session[k] = v
	}
	cli.sessionLock.Unlock()
}

func (cli *Client) SessionSet(key string, val interface{}) {
	cli.sessionLock.Lock()
	defer cli.sessionLock.Unlock()
	if cli.currReq == nil {
		cli.session[key] = val
	} else {
		cli.currReq.SessionSet(key, val)
	}

}

func (cli *Client) SessionGet(key string) interface{} {
	cli.sessionLock.RLock()
	defer cli.sessionLock.RUnlock()
	if cli.currReq == nil {
		if val, ok := cli.session[key]; ok {
			return val
		} else {
			return nil
		}
	} else {
		val := cli.currReq.SessionGet(key)
		return val
	}

}

func (cli *Client) Close(reason string) {
	cli.close(reason, false)
}
