package main

import (
	"context"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/erikdubbelboer/gspt"
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
		Error: func(_ http.ResponseWriter, r *http.Request, status int, reason error) {
			log.Printf("Websocket error from %v: %v %v", r.RemoteAddr, status, reason)
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: false,
	}
	hosters  = newHub()
	shutdown func()
)

func main() {
	gspt.SetProcTitle("WZ-Multihoster2 controller")

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

	log.Println("WZ-Multihoster2 controller starting up...")
	var ctx context.Context
	ctx, shutdown = signal.NotifyContext(context.Background(), os.Interrupt)
	var wg sync.WaitGroup
	waitHandle := func(f func()) {
		wg.Add(1)
		f()
		wg.Done()
	}

	log.Print("Launching websocket hub...")
	go waitHandle(func() { hosters.Run(ctx) })

	log.Print("Launching message processor...")
	go waitHandle(func() { messageProcessor(ctx) })

	log.Print("Launching web server...")
	router := mux.NewRouter()
	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/connect/hoster", handshakeHoster).Methods("GET")
	router.HandleFunc("/shutdown", shutdownHandler).Methods("GET")
	srv := &http.Server{
		Addr:    envOr("LISTEN", "0.0.0.0:4100"),
		Handler: router,
	}
	go waitHandle(func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Web server error: %v", err)
		}
	})
	go waitHandle(func() {
		<-ctx.Done()
		log.Println("Shutting down web server...")
		ctx, srvShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			log.Print("Web server shutdown error", err)
		}
		srvShutdownCancel()
		log.Println("Web server shutdown")
	})

	// log.Print("Launching hoster population controller...")
	spawnNewInstance()

	log.Print("Startup completed")

	<-ctx.Done()
	log.Print("Shutting down...")
	wg.Wait()
	log.Print("Bye!")
}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("Shutdown ordered."))
	shutdown()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("Hello world!"))
}

func handshakeHoster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.Header.Get("HosterID"))
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
		send: make(chan *HosterMsg, 2),
	}
	hosters.connect <- h
}

func messageProcessor(ctx context.Context) {
	for {
		select {
		case m, ok := <-recv:
			if !ok {
				log.Println("Message recv channel was closed")
			}
			log.Print("Message from hoster ", m.id, " ", string(m.content.([]byte)))
			hosters.send <- m
		case <-ctx.Done():
			log.Print("Message processor shutdown")
			return
		}
	}
}

func spawnNewInstance() {
	pid, err := syscall.ForkExec("../hoster/hoster", []string{"-fork"}, &syscall.ProcAttr{
		Dir:   "",
		Env:   []string{},
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Sys: &syscall.SysProcAttr{
			Setpgid:    true,
			Foreground: false,
			Pgid:       0,
			Pdeathsig:  0,
		},
	})
	if err != nil {
		log.Print(err)
	} else {
		log.Printf("Created instance with pid %v", pid)
	}
}
