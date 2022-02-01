package server

import (
	"os"
	"strconv"
	"testing"
)

func TestInitialize(t *testing.T) {
	zipperChan := make(chan string, 1)
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() got err: %v", err)
	}
	from, to := 8000, 8080
	serv, cloase, err := Initialize(wd, zipperChan, from, to)
	if err != nil {
		t.Errorf("Initialize(%s, %p, %d, %d) got err: %v", wd, zipperChan, from, to, err)
	}
	cloase()
	if serv.dir != wd {
		t.Errorf("serv.dir != wd: %s != %s", serv.dir, wd)
	}
	numAvaiablePorts := serv.availablePorts.Len()
	if numAvaiablePorts != to-from+1 {
		t.Errorf("len(serv.availablePorts) != to - from + 1: %d != %d", numAvaiablePorts, to-from+1)
	}

	port := serv.getAvailablePort()
	if serv.availablePorts.Len() != numAvaiablePorts-1 {
		t.Errorf("len(serv.availablePorts) != totalNumberOfAvailablePorts - 1: %d != %d", serv.availablePorts.Len(), numAvaiablePorts-1)
	}
	if port != strconv.Itoa(to) {
		t.Errorf("First port from getAvailablePort is not expected last port %d but instad %s", to, port)
	}
	serv.availablePorts.PushFront(port)
	if serv.availablePorts.Len() != numAvaiablePorts {
		t.Errorf("len(serv.availablePorts) != totalNumberOfAvailablePorts: %d != %d", serv.availablePorts.Len(), numAvaiablePorts)
	}
}

func TestNoTesting(t *testing.T) {
	zipperChan := make(chan string, 1)
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() got err: %v", err)
	}
	from, to := 8000, 8080
	serv, cloase, err := Initialize(wd, zipperChan, from, to)
	if err != nil {
		t.Errorf("Initialize(%s, %p, %d, %d) got err: %v", wd, zipperChan, from, to, err)
	}
	cloase()

	if serv.HasTesting() {
		t.Errorf("Server has a testing when it shouldn't")
	}
}

func TestCheckReliability(t *testing.T) {
	zipperChan := make(chan string, 1)
	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() got err: %v", err)
	}
	from, to := 8000, 8080
	serv, cloase, err := Initialize(wd, zipperChan, from, to)
	if err != nil {
		t.Errorf("Initialize(%s, %p, %d, %d) got err: %v", wd, zipperChan, from, to, err)
	}
	cloase()

	serv.CheckReliability("")
	select {
	case <-serv.serverCommands:
		t.Errorf("Server had a waiting server command after reliability check when there were no servers")
	default:
	}
}
