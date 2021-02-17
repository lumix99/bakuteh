package bakuteh

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	maxDataListSize = 1024
)

type ActionMap map[string]func(*Request)

type ActionClass interface {
	Register(*Server) ActionMap
}

type Bakuteh struct {
	serverMap map[string]*Server
	srvLock   sync.RWMutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func New() *Bakuteh {
	bk := &Bakuteh{
		serverMap: make(map[string]*Server),
		//actions:   make(map[string]ActionMap),
	}

	go bk.daemon()

	return bk
}

func (bk *Bakuteh) HandleServer(path string, act ActionClass) {
	bk.srvLock.Lock()
	defer bk.srvLock.Unlock()

	srv, exist := bk.serverMap[path]
	if exist {
		srv.Stop()
	}

	srv = new(Server)
	srv.actions = act.Register(srv)
	bk.serverMap[path] = srv

	go srv.Run()

}

func (bk *Bakuteh) ServerWs(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	bk.srvLock.RLock()
	defer bk.srvLock.RUnlock()
	srv, exist := bk.serverMap[path]
	if !exist {
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return
	}

	session := make(map[string]interface{})
	qs := r.URL.Query()
	for k, v := range qs {
		session[k] = v
	}

	newClient(srv, conn, session)

}

func (bk *Bakuteh) daemon() {

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-sigs
	fmt.Println(sig)

	bk.srvLock.Lock()
	defer bk.srvLock.Unlock()

	group := sync.WaitGroup{}
	for name, srv := range bk.serverMap {
		group.Add(1)
		go func(s *Server) {
			s.Stop()
			group.Done()
		}(srv)
		delete(bk.serverMap, name)
	}
	group.Wait()
	os.Exit(0)

}
