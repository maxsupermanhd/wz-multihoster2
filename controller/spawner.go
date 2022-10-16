package main

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"
)

var (
	toWatchPid = make(chan int)
)

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
		toWatchPid <- pid
	}
}

func PIDWatcher(ctx context.Context) {
	t := time.NewTimer(5 * time.Second)
	p := map[int]bool{}
l:
	for {
		select {
		case pid := <-toWatchPid:
			log.Println("PID watcher now tracking", pid, "among", len(p), "others")
			p[pid] = true
		case <-t.C:
			for i := range p {
				var usage syscall.Rusage
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(i, &status, syscall.WNOHANG, &usage)
				if err != nil {
					log.Println("PID watcher error:", err)
				}
				if status.Exited() && pid == i {
					log.Println("PID watcher collected", i)
					delete(p, i)
				}
			}
			t.Reset(5 * time.Second)
		case <-ctx.Done():
			log.Println("Pid watcher shutting down...")
			for i := range p {
				var usage syscall.Rusage
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(i, &status, syscall.WNOHANG, &usage)
				if err != nil {
					log.Println("PID watcher error:", err)
				}
				if status.Exited() && pid == i {
					log.Println("PID watcher collected", i)
					delete(p, i)
				}
			}
			break l
		}
	}
	log.Println("PID watcher shutdown.")
}
