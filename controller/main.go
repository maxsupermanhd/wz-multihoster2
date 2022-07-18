package main

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
)

var (
	recv     = make(chan *HosterMsg, 16)
	upgrader = websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		ReadBufferSize:   0,
		WriteBufferSize:  0,
		WriteBufferPool:  nil,
		Subprotocols:     []string{},
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			log.Printf("Websocket error: %v %v", status, reason)
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: false,
	}
	hosters = newHub()
)

func main() {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	rand.Seed(time.Now().UTC().UnixNano())
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename: envOr("LOGDIR", "./logs/") + envOr("LOGFNAME", "controller.log"),
		MaxSize:  10,   // megabytes
		Compress: true, // disabled by default
	}))

	log.Println("Multihoster2 controller starting up...")

	log.Print("Launching websocket hub...")
	go hosters.Run()

	log.Print("Launching message processor...")
	go messageProcessor()

	log.Print("Launching web server...")
	router := mux.NewRouter()
	router.HandleFunc("/connect/hoster", handshakeHoster).Methods("GET")
	srv := &http.Server{
		Addr:    envOr("LISTEN", "0.0.0.0:4100"),
		Handler: router,
	}
	go func() { log.Print(srv.ListenAndServe()) }()

	log.Print("Startup completed")
	<-shutdown
	log.Print("Shutting down web server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Print("Web server shutdown error", err)
	}

	log.Print("Shutting down websocket hub...")
	hosters.Shutdown()

	log.Print("Shutting down message processor...")
	close(recv)
	log.Print("Bye!")

}

func handshakeHoster(w http.ResponseWriter, r *http.Request) {
	sid := r.Header.Get("HosterID")
	if sid == "" {
		log.Print("Empty HosterID http header from ", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(sid)
	if err != nil {
		log.Print("Bad HosterID http header from ", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v (from %v)", err, r.RemoteAddr)
		return
	}
	log.Print("Accepted hoster ", id, " from ", r.RemoteAddr)
	h := &Hoster{
		id:   id,
		conn: c,
		hub:  hosters,
	}
	hosters.connect <- h
}

func messageProcessor() {
	for m := range recv {
		log.Print("Message from hoster ", m.id, string(m.content.([]byte)))
	}
	log.Print("Message processor shutdown")
}
