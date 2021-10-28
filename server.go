package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"sync"
	"time"
)

type serve struct { // TODO rename
	port       string
	errors     int
	warnings   int
	breaking   int
	requests   int
	mesureFrom time.Time
	server     *exec.Cmd
	ctx        context.Context
	once       sync.Once
	kill       func()
}

func (s serve) reliabilityScore(compServ *serve) float64 {
	if time.Now().Sub(s.mesureFrom) < time.Minute*1 {
		return -1
	}
	return s.internalReliabilityScore() - compServ.internalReliabilityScore()
}

func (s serve) internalReliabilityScore() float64 {
	return math.Log2(float64(s.requests) - float64(s.breaking*100+s.errors*10+s.warnings+1))
}

func newServer(path, t string, server **serve) (err error) {
	port := getPort()
	newPath, err := createNewServerInstanceStructure(path, t, port)
	if err != nil {
		availablePorts.PushFront(port)
		return
	}

	s := startNewServer(newPath)
	if s == nil {
		availablePorts.PushFront(port)
		return
	}
	s.port = port

	switch t {
	case "running":
		runningServerLock.Lock()
	case "testing":
		testingServerLock.Lock()
	}
	var oldServer *serve
	oldServer, *server = *server, s
	switch t {
	case "running":
		runningServerLock.Unlock()
	case "testing":
		testingServerLock.Unlock()
	}

	err = symlinkFolder(path, t)
	if oldServer != nil {
		oldServer.kill()
		availablePorts.PushFront(oldServer.port)
	}
	return
}

func startNewServer(serverFolder string) *serve {
	stdOut, err := os.OpenFile(serverFolder+"/stdOut", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
		return nil
	}
	stdErr, err := os.OpenFile(serverFolder+"/stdErr", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "java", "-jar", fmt.Sprintf("%s/%s.jar", serverFolder, os.Getenv("identifier"))) //Look into how to override port in config
	cmd.Dir = serverFolder
	log.Println(cmd)
	err = cmd.Start()
	if err != nil {
		log.Printf("ERROR: Updating server %v\n", err)
		return nil
	}
	pid, err := os.OpenFile(serverFolder+"/pid", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err == nil {
		fmt.Fprintln(pid, cmd.Process.Pid)
		pid.Close()
	}
	time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the service can start without getting any new request, this should not be needed
	server := &serve{
		server: cmd,
		ctx:    ctx,
		kill: func() {
			cancel()
			stdOut.Close()
			stdErr.Close()
		},
	}
	go parseLogServer(server, ctx)
	return server
}
