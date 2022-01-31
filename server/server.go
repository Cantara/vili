package server

import (
	"container/list"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fs"
	"github.com/cantara/vili/server/servlet"
	"github.com/cantara/vili/slack"
	"github.com/cantara/vili/typelib"
)

type commandType int

const (
	newServer commandType = iota
	restartServer
	deployServer
)

type commandData struct {
	server     string
	command    commandType
	serverType typelib.ServerType
}

type Server struct {
	running        servletHandler
	testing        servletHandler
	availablePorts *list.List
	oldFolders     chan<- string
	serverCommands chan commandData
	dir            string
}

type servletHandler struct {
	servlet    *servlet.Servlet
	mesureFrom time.Time
	mutex      sync.Mutex
	isDying    bool
	serverType typelib.ServerType
	dir        string
}

func Initialize(workingDir string, of chan<- string, portrangeFrom, portrangeTo int) (s Server, err error) {
	s = Server{
		running: servletHandler{
			serverType: typelib.RUNNING,
		},
		testing: servletHandler{
			serverType: typelib.TESTING,
		},
		oldFolders:     of,
		serverCommands: make(chan commandData, 5),
		dir:            workingDir,
	}
	s.setAvailablePorts(portrangeFrom, portrangeTo)
	firstServerPath, err := fs.GetFirstServerDir(s.dir, typelib.RUNNING)
	if err != nil {
		return
	}
	log.Debug(firstServerPath)
	err = s.startNewServer(firstServerPath, typelib.RUNNING)
	if err != nil {
		return
	}

	firstTestServerPath, err := fs.GetFirstServerDir(s.dir, typelib.TESTING)
	if err != nil {
		return
	}
	log.Debug(firstTestServerPath)
	if firstServerPath != firstTestServerPath {
		err = s.startNewServer(firstTestServerPath, typelib.TESTING)
	}
	if err != nil {
		return
	}
	go s.NewServerWatcher()
	return
}

func (s *Server) NewServerWatcher() {
	for {
		select {
		case command := <-s.serverCommands:
			switch command.command {
			case newServer: //New servers are always testing
				path, err := fs.CreateNewServerStructure(command.server)
				if err != nil {
					log.AddError(err).Error("Creatubg new server structure")
					continue
				}
				oldFolder := ""
				if s.testing.servlet != nil {
					oldFolder = s.testing.dir
				}
				err = s.startNewServer(path, typelib.TESTING)
				if oldFolder != "" {
					s.oldFolders <- oldFolder
				}
			case restartServer:
				log.Info("RESTARTING ", command.serverType)
				var err error
				switch command.serverType {
				case typelib.RUNNING:
					s.running.mutex.Lock()
					if s.running.isDying {
						s.running.mutex.Unlock()
						continue
					}
					s.running.isDying = true
					s.running.mutex.Unlock()

					err = s.startNewServer(s.running.dir, command.serverType)
				case typelib.TESTING:
					s.testing.mutex.Lock()
					if s.testing.isDying || s.testing.servlet == nil {
						s.testing.mutex.Unlock()
						continue
					}
					s.testing.isDying = true
					s.testing.mutex.Unlock()

					err = s.startNewServer(s.testing.dir, command.serverType)
				}
				if err != nil {
					log.AddError(err).Error("Restarting server ", command.serverType)
				}
			case deployServer:
				log.Info("DEPLOYING NEW RUNNING SERVER")
				s.testing.mutex.Lock()
				if s.testing.servlet == nil {
					log.Info("Nothing to deploy")
					s.testing.mutex.Unlock()
					return
				}
				server := s.testing.dir
				s.testing.servlet.Kill()
				s.availablePorts.PushFront(s.testing.servlet.Port)
				s.testing.servlet = nil
				s.testing.mutex.Unlock()

				oldFolder := s.running.dir
				err := s.startNewServer(server, typelib.RUNNING)
				if err != nil {
					log.AddError(err).Error("New server deployment")
				}
				s.oldFolders <- oldFolder
			}
		}
	}
}

func (s *Server) NewTesting(server string) {
	s.serverCommands <- commandData{command: newServer, server: server}
}

func (s *Server) Deploy() {
	s.serverCommands <- commandData{command: deployServer}
}

func (s *Server) Restart(t typelib.ServerType) {
	s.serverCommands <- commandData{command: restartServer, serverType: t}
}

func (s Server) reliabilityScore() float64 {
	if s.TestingDuration() < time.Minute*1 {
		return -1
	}
	return s.testing.servlet.ReliabilityScore() - s.running.servlet.ReliabilityScore()
}

func (s *Server) startNewServer(path string, t typelib.ServerType) (err error) {
	path = strings.TrimRight(path, "/")
	port := s.getAvailablePort()
	newPath, err := fs.CreateNewServerInstanceStructure(path, t, port)
	if err != nil {
		s.availablePorts.PushFront(port)
		return
	}

	serv, err := servlet.NewServlet(newPath, port)
	//s := startNewServer(newPath, port)
	if err != nil {
		log.AddError(err).Warning("While creating new server")
		s.availablePorts.PushFront(port)
		return
	}

	var oldServer *servlet.Servlet
	switch t {
	case typelib.RUNNING:
		s.running.mutex.Lock()
		oldServer, s.running.servlet = s.running.servlet, &serv
		s.running.dir = path
		s.running.isDying = false
		s.running.mutex.Unlock()
	case typelib.TESTING:
		s.testing.mutex.Lock()
		oldServer, s.testing.servlet = s.testing.servlet, &serv
		s.testing.dir = path
		s.testing.isDying = false
		s.testing.mutex.Unlock()
	}

	err = fs.SymlinkFolder(path, t)
	if oldServer != nil {
		oldServer.Kill()
		s.availablePorts.PushFront(oldServer.Port)
	}
	s.ResetTest()
	go s.watchServerStatus(t, &serv)
	return
}

func (s *Server) watchServerStatus(t typelib.ServerType, serv *servlet.Servlet) {
	//sleepInterval := 1
	for {
		time.Sleep(1 * time.Minute)
		if !serv.IsRunning() {
			s.Restart(t)
			return
		}
	}
}

func (s Server) GetRunningVersion() string {
	path := strings.Split(s.running.dir, "/")
	return path[len(path)-1]
}

func (s Server) GetPort(t typelib.ServerType) string {
	switch t {
	case typelib.RUNNING:
		return s.running.servlet.Port
	case typelib.TESTING:
		return s.testing.servlet.Port
	}
	return ""
}

func (s *Server) AddBreaking(t typelib.ServerType) {
	switch t {
	case typelib.RUNNING:
		s.running.servlet.IncrementBreaking()
	case typelib.TESTING:
		s.testing.servlet.IncrementBreaking()
	}
}

func (s *Server) AddRequest(t typelib.ServerType) {
	switch t {
	case typelib.RUNNING:
		s.running.servlet.IncrementRequests()
	case typelib.TESTING:
		s.testing.servlet.IncrementRequests()
	}
}

func (s *Server) HasTesting() bool {
	s.testing.mutex.Lock()
	defer s.testing.mutex.Unlock()
	return s.testing.servlet != nil
}

func (s *Server) TestingDuration() time.Duration {
	s.testing.mutex.Lock()
	defer s.testing.mutex.Unlock()
	if s.testing.servlet == nil {
		return time.Duration(0)
	}
	return time.Now().Sub(s.testing.mesureFrom)
}

func (s *Server) Messuring() bool {
	if s.testing.servlet == nil {
		return false
	}
	s.testing.mutex.Lock()
	defer s.testing.mutex.Unlock()
	return !s.testing.mesureFrom.IsZero()
}

func (s *Server) ResetTest() { //TODO: Make better
	if !s.HasTesting() {
		return
	}
	s.testing.mutex.Lock()
	s.testing.mesureFrom = time.Now()
	s.testing.servlet.ResetTestData()
	s.testing.mutex.Unlock()
	s.running.mutex.Lock()
	s.running.mesureFrom = time.Now()
	s.running.servlet.ResetTestData()
	s.running.mutex.Unlock()
}

func (s Server) getAvailablePort() string {
	port := s.availablePorts.Front()
	s.availablePorts.Remove(port)
	return port.Value.(string)
}

func (s *Server) setAvailablePorts(from, to int) {
	if s.availablePorts != nil {
		return
	}
	s.availablePorts = list.New()
	for i := from; i <= to; i++ {
		s.availablePorts.PushFront(strconv.Itoa(i))
	}
}

func (s Server) IsRunning(t typelib.ServerType) bool {
	switch t {
	case typelib.RUNNING:
		return s.running.servlet.IsRunning()
	case typelib.TESTING:
		if s.testing.servlet == nil {
			return false
		}
		return s.testing.servlet.IsRunning()
	}
	return false
}

func (s *Server) CheckReliability(hostname string) {
	log.Println("reliabilityScore of testingServer compared to runningServer: ", s.reliabilityScore())
	if s.reliabilityScore() >= -0.25 {
		s.testing.mutex.Lock()
		if s.testing.isDying || s.testing.servlet == nil {
			s.testing.mutex.Unlock()
			return
		}
		s.testing.isDying = true
		s.testing.mutex.Unlock()
		go slack.Sendf(" :hourglass: Vili started switching to new version host: %s, new version %s.", hostname, s.GetRunningVersion())
		s.Deploy()
		go slack.Sendf(" :+1:  Vili switch to new version complete on host: %s, version %s.", hostname, s.GetRunningVersion())
	}
}

func (s *Server) Kill() {
	s.testing.mutex.Lock()
	if s.testing.servlet != nil {
		s.testing.servlet.Kill()
	}
	s.testing.mutex.Unlock()
	s.running.mutex.Lock()
	if s.running.servlet != nil {
		s.running.servlet.Kill()
	}
	s.running.mutex.Unlock()
}
