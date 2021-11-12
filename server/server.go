package server

import (
	"container/list"
	"strconv"
	"sync"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fs"
	"github.com/cantara/vili/server/servlet"
	"github.com/cantara/vili/typelib"
)

type Server struct {
	running        servletHandler
	testing        servletHandler
	availablePorts *list.List
	oldFolders     chan<- string
	dir            string
}

type servletHandler struct {
	servlet    *servlet.Servlet
	mesureFrom time.Time
	mutex      sync.Mutex
	once       sync.Once
	serverType typelib.ServerType
	dir        string
}

func NewServer(workingDir string, of chan<- string, portrangeFrom, portrangeTo int) (s Server, err error) {
	s = Server{
		running: servletHandler{
			serverType: typelib.RUNNING,
		},
		testing: servletHandler{
			serverType: typelib.TESTING,
		},
		oldFolders: of,
		dir:        workingDir,
	}
	s.setAvailablePorts(portrangeFrom, portrangeTo)
	firstServerPath, err := fs.GetFirstServerDir(s.dir, typelib.RUNNING)
	if err != nil {
		return
	}
	log.Debug(firstServerPath)
	err = s.newServer(firstServerPath, typelib.RUNNING)
	if err != nil {
		return
	}

	firstTestServerPath, err := fs.GetFirstServerDir(s.dir, typelib.TESTING)
	if err != nil {
		return
	}
	log.Debug(firstTestServerPath)
	if firstServerPath != firstTestServerPath {
		err = s.newServer(firstTestServerPath, typelib.TESTING)
	}
	return
}

func (s Server) reliabilityScore() float64 {
	if s.testing.servlet == nil || time.Now().Sub(s.testing.mesureFrom) < time.Minute*1 {
		return -1
	}
	return s.testing.servlet.ReliabilityScore() - s.running.servlet.ReliabilityScore()
}

func (s *Server) newServer(path string, t typelib.ServerType) (err error) {
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
		s.running.once = *new(sync.Once)
		s.running.mutex.Unlock()
	case typelib.TESTING:
		s.testing.mutex.Lock()
		oldServer, s.testing.servlet = s.testing.servlet, &serv
		s.testing.dir = path
		s.testing.once = *new(sync.Once)
		s.testing.mutex.Unlock()
	}

	err = fs.SymlinkFolder(path, t)
	if oldServer != nil {
		oldServer.Kill()
		s.availablePorts.PushFront(oldServer.Port)
	}
	return
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
	if !s.Messuring() {
		s.testing.mutex.Lock()
		s.testing.mesureFrom = time.Now()
		s.testing.servlet.ResetTestData()
		s.testing.mutex.Unlock()
		s.running.mutex.Lock()
		s.running.mesureFrom = time.Now()
		s.running.servlet.ResetTestData()
		s.running.mutex.Unlock()
	}
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

func (s *Server) Deploy() {
	log.Println("DEPLOYING NEW RUNNING SERVER")
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
	err := s.newServer(server, typelib.RUNNING)
	if err != nil {
		log.AddError(err).Debug("New server deployment")
	}
	s.oldFolders <- oldFolder
	return
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

func (s *Server) restart(t typelib.ServerType) (err error) {
	log.Println("RESTARTING ", t)
	switch t {
	case typelib.RUNNING:
		err = s.newServer(s.running.dir, t)
	case typelib.TESTING:
		if s.testing.servlet == nil {
			return
		}
		err = s.newServer(s.testing.dir, t)
	}
	return
}
func (s *Server) Restart(t typelib.ServerType) {
	r := func() {
		err := s.restart(t)
		log.AddError(err).Warning("While restarting server: ", t)
	}
	switch t {
	case typelib.RUNNING:
		s.running.once.Do(r)
	case typelib.TESTING:
		s.testing.once.Do(r)
	}
}

func (s *Server) NewTesting(serv string) (err error) {
	path, err := fs.CreateNewServerStructure(serv)
	if err != nil {
		return
	}
	oldFolder := ""
	if s.testing.servlet != nil {
		oldFolder = s.testing.dir
	}
	err = s.newServer(path, typelib.TESTING)
	if oldFolder != "" {
		s.oldFolders <- oldFolder
	}
	return
}

func (s *Server) CheckReliability() {
	log.Println("reliabilityScore of testingServer compared to runningServer: ", s.reliabilityScore())
	if s.reliabilityScore() >= -0.25 {
		s.testing.once.Do(s.Deploy)
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
