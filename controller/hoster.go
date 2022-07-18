package main

import (
	"log"

	"github.com/gorilla/websocket"
)

type HosterMsg struct {
	id      int
	content interface{}
}

type Hoster struct {
	id   int
	conn *websocket.Conn
	hub  *WSHub
	send chan *HosterMsg
}

func (h *Hoster) ReadPump() {
	for {
		_, m, err := h.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, 1000) {
				log.Print("Error reading from hoster", err)
			} else {
				log.Print("Hoster ", h.id, " dissconnected")
			}
			break
		}
		recv <- &HosterMsg{
			id:      h.id,
			content: m,
		}
	}
}
