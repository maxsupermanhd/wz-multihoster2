package main

import (
	"context"
	"log"
	"sync"
)

type WSHub struct {
	clients    map[*Hoster]int
	connect    chan *Hoster
	disconnect chan *Hoster
	send       chan *HosterMsg
	lock       sync.Mutex
}

func newHub() *WSHub {
	return &WSHub{
		clients:    map[*Hoster]int{},
		connect:    make(chan *Hoster),
		disconnect: make(chan *Hoster),
		send:       make(chan *HosterMsg),
	}
}

func (h *WSHub) Run(ctx context.Context) {
	if !h.lock.TryLock() {
		log.Println("Failed to lock hub!")
	}
	defer h.lock.Unlock()

	var wg sync.WaitGroup

	for {
		select {
		case c := <-h.connect:
			h.clients[c] = c.id
			wg.Add(2)
			go func() {
				c.ReadPump()
				wg.Done()
			}()
			go func() {
				c.WritePump()
				wg.Done()
			}()
		case client := <-h.disconnect:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.send:
			sent := false
			for client, id := range h.clients {
				if id == message.id {
					if client.send == nil {
						log.Println("Client send pipe is nil!")
					} else {
						sent = true
						client.send <- message
					}
					break
				}
			}
			if !sent {
				log.Print("Message ", message, " was not delivered")
			}
		case <-ctx.Done():
			log.Print("Shutting down websocket hub...")
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			close(h.connect)
			close(h.disconnect)
			close(h.send)
			log.Print("Closing websocket connections...")
			wg.Wait()
			log.Print("Websocket connections closed")
			return
		}
	}
}

func (h *WSHub) Send(id int, content interface{}) {
	h.send <- &HosterMsg{
		id:      id,
		content: content,
	}
}
