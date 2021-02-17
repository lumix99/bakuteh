package bakuteh

const (
	ACTION_INIT  = "init"
	ACTION_ERROR = "error"
	ACTION_CLOSE = "close"
)

const (
	signalRegister = iota
	signalUnregister
	signalUnregisterP
	signalRoomjoin
	signalRoomQuit
	signalRoomBroadcast
	signalBroadcast
	signalStop
	signalSend
)

type Message struct {
	ID     int         `json:"id"`
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

type Signal struct {
	Action int
	Cli    *Client
	Room   string
	Msg    *Message
	Err    chan error
}
