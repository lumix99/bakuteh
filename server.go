package bakuteh

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type Server struct {
	accepter  chan *Signal
	clients   map[*Client]bool
	actions   ActionMap
	run       sync.Once
	closed    bool
	rooms     map[string]map[*Client]bool
	DelayQuit time.Duration
}

func (srv *Server) Run() {
	srv.run.Do(func() {
		srv.accepter = make(chan *Signal, 10)
		srv.clients = make(map[*Client]bool)
		srv.rooms = make(map[string]map[*Client]bool)
		srv.closed = false

		for sgl := range srv.accepter {
			if srv.closed {
				if sgl.Action == signalUnregister || sgl.Action == signalUnregisterP {
					srv.clientQuit(sgl)
				}
				if sgl.Err != nil {
					sgl.Err <- errors.New("server is closed")
				}
				if len(srv.clients) == 0 {
					close(srv.accepter)
				}

				continue
			}

			switch sgl.Action {
			case signalRegister:
				srv.clients[sgl.Cli] = true
			case signalUnregister, signalUnregisterP:
				srv.clientQuit(sgl)
			case signalRoomjoin:
				if _, ok := srv.rooms[sgl.Room]; !ok {
					srv.rooms[sgl.Room] = make(map[*Client]bool)
				}
				srv.rooms[sgl.Room][sgl.Cli] = true
			case signalRoomQuit:
				delete(srv.rooms[sgl.Room], sgl.Cli)
			case signalBroadcast, signalRoomBroadcast:
				srv.broadcast(sgl)
			case signalStop:
				srv.stop()
			}

			if sgl.Err != nil {
				close(sgl.Err)
			}
		}

	})
}

func (srv *Server) Broadcast(msg *Message, chs ...chan error) {
	sgl := &Signal{Action: signalBroadcast, Msg: msg}
	if len(chs) == 1 {
		sgl.Err = chs[0]
	}

	srv.accepter <- sgl
}

func (srv *Server) RoomJoin(name string, cli *Client) {
	srv.accepter <- &Signal{Action: signalRoomjoin, Room: name, Cli: cli}

}

func (srv *Server) RoomQuit(name string, cli *Client) {
	srv.accepter <- &Signal{Action: signalRoomQuit, Room: name, Cli: cli}
}

func (srv *Server) Stop() {
	sgl := &Signal{
		Action: signalStop,
		Err:    make(chan error),
	}
	srv.accepter <- sgl
	err := <-sgl.Err
	if err != nil {
		fmt.Println("server stop error:", err)
	}
}

func (srv *Server) clientQuit(sgl *Signal) {
	cli := sgl.Cli
	passive := false
	if sgl.Action == signalUnregisterP {
		passive = true
	}

	alive, ok := srv.clients[cli]
	if !ok {
		return
	}
	if alive && srv.DelayQuit != 0 && passive {
		srv.clients[cli] = false
		go func() {
			time.Sleep(srv.DelayQuit)
			srv.accepter <- &Signal{Action: signalUnregister, Cli: cli}
		}()
		return
	}

	for _, clis := range srv.rooms {
		delete(clis, cli)
	}

	delete(srv.clients, cli)
}

func (srv *Server) RoomBroadcast(room string, msg *Message, chs ...chan error) {
	sgl := &Signal{Action: signalRoomBroadcast, Room: room, Msg: msg}
	if len(chs) == 1 {
		sgl.Err = chs[0]
	}

	srv.accepter <- sgl
}

func (srv *Server) broadcast(sgl *Signal) {
	var clis map[*Client]bool
	if sgl.Action == signalBroadcast {
		clis = srv.clients
	} else if sgl.Action == signalRoomBroadcast {
		clis = srv.rooms[sgl.Room]
	}
	if len(clis) == 0 {
		return
	}

	if sgl.Err != nil {
		group := sync.WaitGroup{}
		for c, ok := range clis {
			if ok {
				group.Add(1)
				go func() {
					ch := make(chan error)
					c.Send(sgl.Msg, ch)
					<-ch
					group.Done()
				}()

			}
		}
		group.Wait()
		return
	}

	for c, ok := range clis {
		if ok {
			c.Send(sgl.Msg)
		}
	}

}

func (srv *Server) stop() {
	srv.closed = true
	group := sync.WaitGroup{}
	for cli := range srv.clients {
		group.Add(1)
		go func(c *Client) {
			c.Close("")
			group.Done()
		}(cli)
	}

	group.Wait()
}
