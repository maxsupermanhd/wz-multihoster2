package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type HosterMsg struct {
	id      int
	content interface{}
}

type Hoster struct {
	ID    int
	State string
	conn  *websocket.Conn
	hub   *WSHub
	send  chan *HosterMsg
}

func (h *Hoster) ReadPump() {
	for {
		_, m, err := h.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, 1000) {
				log.Print("Error reading from hoster: ", err)
			} else {
				log.Print("Hoster ", h.ID, " dissconnected")
			}
			break
		}
		recv <- &HosterMsg{
			id:      h.ID,
			content: m,
		}
	}
}

func (h *Hoster) WritePump() {
	for msg := range h.send {
		content, err := json.Marshal(msg.content)
		if err != nil {
			log.Printf("Failed to marshal message: %v\n", err.Error())
			continue
		}
		err = h.conn.WriteMessage(websocket.TextMessage, content)
		if err != nil {
			log.Printf("Failed to send message: %v\n", err.Error())
		}
	}
	err := h.conn.WriteMessage(websocket.CloseMessage, []byte{})
	if err != nil {
		log.Printf("Client %d failed to send close message: %s", h.ID, err)
	}
	h.conn.Close()
}
