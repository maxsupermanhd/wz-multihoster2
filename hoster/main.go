package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	doReFork = flag.Bool("fork", false, "Will cause hoster to re-fork")
)

func main() {
	flag.Parse()
	if *doReFork {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		c := exec.Command(ex)
		c.Start()
		time.Sleep(1 * time.Second)
		return
	}
	log.Print("Hoster instance started")
	interrupt := make(chan os.Signal, 256)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGHUP)

	log.Print("Connecting...")
	u := url.URL{
		Scheme: "ws",
		Host:   "localhost:4100",
		Path:   "/connect/hoster",
	}

	for {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), map[string][]string{"HosterID": {fmt.Sprint(time.Now().Unix())}})
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer c.Close()
		log.Print("Connected")
		done := make(chan struct{})
		recv := make(chan []byte, 16)
		go func() {
			defer close(done)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					return
				}
				recv <- message
			}
		}()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case r := <-recv:
				log.Print("Recieved ", string(r))
			case t := <-ticker.C:
				err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
				if err != nil {
					log.Println("write:", err)
					break
				}
			case i := <-interrupt:
				if i != syscall.SIGHUP {
					log.Println("interrupt")

					// Cleanly close the connection by sending a close message and then
					// waiting (with timeout) for the server to close the connection.
					err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("write close:", err)
						return
					}
					select {
					case <-done:
					case <-time.After(time.Second):
					}
					return
				}
			}
		}
	}

}
