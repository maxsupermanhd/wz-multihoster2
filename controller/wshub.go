package main

import "log"

type WSHub struct {
	clients    map[*Hoster]int
	connect    chan *Hoster
	disconnect chan *Hoster
	send       chan *HosterMsg
	shutdown   chan struct{}
}

func newHub() *WSHub {
	return &WSHub{
		clients:    map[*Hoster]int{},
		connect:    make(chan *Hoster),
		disconnect: make(chan *Hoster),
		send:       make(chan *HosterMsg),
		shutdown:   make(chan struct{}),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case c := <-h.connect:
			h.clients[c] = c.id
			go c.ReadPump()
		case client := <-h.disconnect:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.send:
			sent := false
			for client, id := range h.clients {
				if id == message.id && client.send != nil {
					sent = true
					select {
					case client.send <- message:
					default:
						delete(h.clients, client)
					}
					break
				}
			}
			if !sent {
				log.Print("Message", message, "was not delivered")
			}
		case <-h.shutdown:
			log.Print("Shutting down websocket hub...")
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			close(h.connect)
			close(h.disconnect)
			close(h.send)
			defer close(h.shutdown)
			log.Print("Websocket connections closed")
			return
		}
	}
}

func (h *WSHub) Shutdown() {
	h.shutdown <- struct{}{}
	select {
	case <-h.shutdown:
	default:
	}
}

func (h *WSHub) Send(id int, content interface{}) {
	h.send <- &HosterMsg{
		id:      id,
		content: content,
	}
}
