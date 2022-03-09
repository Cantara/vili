package server

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fs"
	"github.com/cantara/vili/fslib"
	"github.com/cantara/vili/server/servlet"
	"github.com/cantara/vili/slack"
	"github.com/cantara/vili/typelib"
)

type commandType int

const (
	newServer commandType = iota
	startServer
	newService
	restartServer
	deployServer
)

type commandData struct {
	server     string
	serverDir  fslib.Dir
	command    commandType
	serverType typelib.ServerType
}

type Server struct {
	running        servletHandler
	testing        servletHandler
	availablePorts *list.List
	oldFolders     chan<- fslib.Dir
	serverCommands chan commandData
	dir            fslib.Dir
	cancel         func()
}

type servletHandler struct {
	servlet    *servlet.Servlet
	mesureFrom time.Time
	mutex      sync.Mutex
	isDying    bool
	serverType typelib.ServerType
	dir        fslib.Dir
}

func Initialize(workingDir fslib.Dir, of chan<- fslib.Dir, portrangeFrom, portrangeTo int) (s Server, err error) {
	ctx, cancel := context.WithCancel(context.Background())
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
		cancel:         cancel,
	}
	s.setAvailablePorts(portrangeFrom, portrangeTo)

	go s.newServerWatcher(ctx)
	return
}

func (s *Server) StartExcistingRunning() (err error) {
	firstServerDir, err := fs.GetFirstServerDir(typelib.RUNNING)
	if err != nil {
		return
	}
	log.Debug(firstServerDir.Path())
	s.startService(firstServerDir, typelib.RUNNING)
	/*err = s.startService(firstServerDir, typelib.RUNNING)
	if err != nil {
		return
	}*/
	return
}

func (s *Server) StartExcistingTesting() (err error) {
	firstTestServerDir, err := fs.GetFirstServerDir(typelib.TESTING)
	if err != nil {
		return
	}
	log.Debug(firstTestServerDir.Path())
	if s.running.dir.Path() != firstTestServerDir.Path() {
		s.startService(firstTestServerDir, typelib.TESTING)
		/*err = s.startService(firstTestServerDir, typelib.TESTING)
		if err != nil {
			return
		}*/
	}
	return
}

func (s *Server) newServerWatcher(ctx context.Context) {
	for {
		select {
		case command := <-s.serverCommands:
			log.Info("New command recieved")
			switch command.command {
			case newServer: //New servers are always testing
				_, err := fs.CreateNewServerStructure(command.server)
				if err != nil {
					log.AddError(err).Error("Creatubg new server structure")
					continue
				}
				var oldFolder fslib.Dir
				if s.testing.servlet != nil {
					oldFolder = s.testing.dir
				}
				if oldFolder != nil {
					s.oldFolders <- oldFolder
				}
				//err = s.startService(path, typelib.TESTING)
			case startServer:
				port := s.getAvailablePort()
				newPath, err := fs.CreateNewServerInstanceStructure(command.serverDir, command.serverType, port)
				if err != nil {
					s.availablePorts.PushFront(port)
					continue
				}

				serv, err := servlet.NewServlet(newPath, port)
				if err != nil {
					log.AddError(err).Warning("While creating new server")
					s.availablePorts.PushFront(port)
					continue
				}

				var oldServer *servlet.Servlet
				switch command.serverType {
				case typelib.RUNNING:
					s.running.mutex.Lock()
					oldServer, s.running.servlet = s.running.servlet, &serv
					s.running.dir = command.serverDir
					s.running.isDying = false
					s.running.mutex.Unlock()
				case typelib.TESTING:
					s.testing.mutex.Lock()
					oldServer, s.testing.servlet = s.testing.servlet, &serv
					s.testing.dir = command.serverDir
					s.testing.isDying = false
					s.testing.mutex.Unlock()
				}

				err = command.serverDir.Symlink(command.serverDir.File(), fmt.Sprintf("%s-%s", os.Getenv("identifier"), command.serverType.String()))
				if oldServer != nil {
					oldServer.Kill()
					s.availablePorts.PushFront(oldServer.Port)
				}
				s.ResetTest()
				//go s.watchServerStatus(t, &serv)
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

					//err = s.startService(s.running.dir, command.serverType)
				case typelib.TESTING:
					s.testing.mutex.Lock()
					if s.testing.isDying || s.testing.servlet == nil {
						s.testing.mutex.Unlock()
						continue
					}
					s.testing.isDying = true
					s.testing.mutex.Unlock()

					//err = s.startService(s.testing.dir, command.serverType)
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
				//server := s.testing.dir
				s.testing.servlet.Kill()
				s.availablePorts.PushFront(s.testing.servlet.Port)
				s.testing.servlet = nil
				s.testing.mutex.Unlock()

				oldFolder := s.running.dir
				/*err := s.startService(server, typelib.RUNNING)
				if err != nil {
					log.AddError(err).Error("New server deployment")
				}*/
				s.oldFolders <- oldFolder
			}
		case <-ctx.Done():
			return
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
	//s.serverCommands <- commandData{command: restartServer, serverType: t}
}

func (s *Server) newVersion(server string, t typelib.ServerType) {
	s.serverCommands <- commandData{command: newService, server: server, serverType: t}
}

func (s *Server) startService(serverDir fslib.Dir, t typelib.ServerType) {
	s.serverCommands <- commandData{command: startServer, serverDir: serverDir, serverType: t}
}

func (s Server) reliabilityScore() float64 {
	if s.TestingDuration() < time.Minute*1 {
		return -1
	}
	return s.testing.servlet.ReliabilityScore() - s.running.servlet.ReliabilityScore()
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
	if s.running.dir == nil {
		return "unknown"
	}
	return s.running.dir.File().Name()
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
	s.cancel()
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
