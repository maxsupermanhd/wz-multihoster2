package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/gorilla/websocket"
)

var (
	doReFork = flag.Bool("fork", false, "Will cause hoster to re-fork")
)

func main() {
	flag.Parse()
	if *doReFork {
		refork()
		return
	}
	log.Print("Hoster instance started")
	gspt.SetProcTitle("WZ-Multihoster2 hoster")
	interrupt := make(chan os.Signal, 256)
	signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGHUP)

	log.Print("Connecting...")
	u := url.URL{
		Scheme: "ws",
		Host:   "localhost:4100",
		Path:   "/connect/hoster",
	}

	hosterid := fmt.Sprint(time.Now().Unix())

	for {
		log.Printf("Dialing %v", u)
		c, _, err := websocket.DefaultDialer.Dial(u.String(), map[string][]string{"HosterID": {hosterid}})
		if err != nil {
			log.Print("dial: ", err)
			log.Println("Reconnecting...")
			time.Sleep(1 * time.Second)
			continue
		}
		defer c.Close()
		log.Print("Connected")
		recv := make(chan []byte, 16)
		go func() {
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

		reconnect := false
		for !reconnect {
			select {
			case r := <-recv:
				log.Print("Recieved ", string(r))
			case t := <-ticker.C:
				err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
				if err != nil {
					log.Println("write:", err)
					reconnect = true
				}
			case i := <-interrupt:
				if i != syscall.SIGHUP {
					err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("write close:", err)
					}
					return
				}
			}
		}
		log.Println("Reconnecting...")
		time.Sleep(1 * time.Second)
	}

}

func refork() {
	gspt.SetProcTitle("WZ-Multihoster2 reforker")
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	fin, err := os.Open("/dev/null")
	if err != nil {
		log.Printf("Failed to open /dev/null: %v", err.Error())
		return
	}
	fout, err := os.OpenFile("/dev/null", os.O_WRONLY, 0777)
	if err != nil {
		log.Printf("Failed to open /dev/null: %v", err.Error())
		return
	}
	ferr, err := os.OpenFile("/dev/null", os.O_WRONLY, 0777)
	if err != nil {
		log.Printf("Failed to open /dev/null: %v", err.Error())
		return
	}
	pid, err := syscall.ForkExec(ex, []string{}, &syscall.ProcAttr{
		Dir:   "",
		Env:   []string{},
		Files: []uintptr{fin.Fd(), fout.Fd(), ferr.Fd()},
		Sys: &syscall.SysProcAttr{
			Setpgid:    true,
			Foreground: false,
			Pgid:       0,
			Pdeathsig:  0,
		},
	})
	if err != nil {
		fmt.Printf("Failed to start: %v\n", err.Error())
	} else {
		fmt.Printf("Started with PID %d\n", pid)
	}
	time.Sleep(1 * time.Second)
	return
}
