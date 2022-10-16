package main

import (
	"context"
	"log"
	"sync"
)

type WSHub struct {
	clients     map[int]*Hoster
	clientsLock sync.RWMutex
	connect     chan *Hoster
	disconnect  chan int
	send        chan *HosterMsg
	runlock     sync.Mutex
}

func newHub() *WSHub {
	return &WSHub{
		clients:    map[int]*Hoster{},
		connect:    make(chan *Hoster),
		disconnect: make(chan int),
		send:       make(chan *HosterMsg),
	}
}

func (h *WSHub) Run(ctx context.Context) {
	if !h.runlock.TryLock() {
		log.Println("Failed to lock hub!")
	}
	defer h.runlock.Unlock()

	var wg sync.WaitGroup

	for {
		select {
		case c := <-h.connect:
			h.clientsLock.Lock()
			h.clients[c.ID] = c
			h.clientsLock.Unlock()
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
			h.clientsLock.Lock()
			if c, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(c.send)
			}
			h.clientsLock.Unlock()
		case message := <-h.send:
			h.clientsLock.RLock()
			h.clients[message.id].send <- message
			h.clientsLock.RUnlock()
		case <-ctx.Done():
			log.Print("Shutting down websocket hub...")
			h.clientsLock.Lock()
			for id, client := range h.clients {
				close(client.send)
				delete(h.clients, id)
			}
			h.clientsLock.Unlock()
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
