package controller

import (
	"fmt"

	"github.com/lumix99/bakuteh"
)

type Chat struct {
	Srv *bakuteh.Server
}

func (c *Chat) Register(srv *bakuteh.Server) bakuteh.ActionMap {
	c.Srv = srv
	actMap := make(bakuteh.ActionMap)
	actMap["init"] = c.Init
	actMap["chat"] = c.Chat
	actMap["close"] = c.Close

	return actMap
}

func (c *Chat) Init(req *bakuteh.Request) {

	room := req.SessionGet("room")
	if room, ok := room.([]string); ok {
		str := room[0]
		req.SessionSet("room", str)

		c.Srv.RoomJoin(str, req.Cli)
	} else {
		req.Cli.Close("no room")
	}

	username := req.SessionGet("username")
	if username, ok := username.([]string); ok {
		str := username[0]
		if len(str) > 0 {
			req.SessionSet("username", str)
			return
		}
	}
	req.SessionSet("username", "nobody")

}

func (c *Chat) Chat(req *bakuteh.Request) {
	room, ok := req.SessionGet("room").(string)
	if !ok {
		req.Response(bakuteh.Message{Action: bakuteh.ACTION_ERROR, Data: "without room"})
		return
	}

	username := req.SessionGet("username").(string)
	data := make(map[string]string)
	data["user"] = username
	data["msg"] = req.Msg.Data.(string)
	msg := &bakuteh.Message{
		ID:     0,
		Action: "chat",
		Data:   data,
	}
	c.Srv.RoomBroadcast(room, msg)
}

func (c *Chat) Close(req *bakuteh.Request) {
	username := req.SessionGet("username").(string)
	fmt.Println("user leave", username)
}
