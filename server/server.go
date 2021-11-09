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

var running ServletHandler
var testing ServletHandler
var availablePorts *list.List
var oldFolders chan<- string

type ServletHandler struct {
	servlet    *servlet.Servlet
	mesureFrom time.Time
	mutex      sync.Mutex
	once       sync.Once
	serverType typelib.ServerType
}

func (s ServletHandler) reliabilityScore(compServ servlet.Servlet) float64 {
	if time.Now().Sub(s.mesureFrom) < time.Minute*1 {
		return -1
	}
	return s.servlet.ReliabilityScore() - compServ.ReliabilityScore()
}

func newServer(path string, t typelib.ServerType, servletHandler *ServletHandler) (err error) {
	port := getPort()
	newPath, err := fs.CreateNewServerInstanceStructure(path, t, port)
	if err != nil {
		availablePorts.PushFront(port)
		return
	}

	s, err := servlet.NewServlet(newPath, port)
	//s := startNewServer(newPath, port)
	if err != nil {
		log.AddError(err).Warning("While creating new server")
		availablePorts.PushFront(port)
		return
	}

	servletHandler.mutex.Lock()
	var oldServer *servlet.Servlet
	oldServer, servletHandler.servlet = servletHandler.servlet, &s
	servletHandler.mutex.Unlock()

	err = fs.SymlinkFolder(path, t)
	if oldServer != nil {
		oldServer.Kill()
		availablePorts.PushFront(oldServer.Port)
	}
	return
}

func GetPort(t typelib.ServerType) string {
	switch t {
	case typelib.RUNNING:
		return running.servlet.Port
	case typelib.TESTING:
		return testing.servlet.Port
	}
	return ""
}

func AddBreaking(t typelib.ServerType) {
	switch t {
	case typelib.RUNNING:
		running.servlet.IncrementBreaking()
	case typelib.TESTING:
		testing.servlet.IncrementBreaking()
	}
}

func AddRequest(t typelib.ServerType) {
	switch t {
	case typelib.RUNNING:
		running.servlet.IncrementRequests()
	case typelib.TESTING:
		testing.servlet.IncrementRequests()
	}
}

func HasTesting() bool {
	testing.mutex.Lock()
	defer testing.mutex.Unlock()
	return testing.servlet != nil
}

func Messuring() bool {
	if testing.servlet == nil {
		return false
	}
	testing.mutex.Lock()
	defer testing.mutex.Unlock()
	return !testing.mesureFrom.IsZero()
}

func ResetTest() { //TODO: Make better
	if !HasTesting() {
		return
	}
	if !Messuring() {
		testing.mesureFrom = time.Now()
		testing.servlet.ResetTestData()
		running.mesureFrom = time.Now()
		running.servlet.ResetTestData()
	}
}

func getPort() string {
	port := availablePorts.Front()
	availablePorts.Remove(port)
	return port.Value.(string)
}

func SetAvailablePorts(from, to int) {
	if availablePorts != nil {
		return
	}
	availablePorts = list.New()
	for i := from; i <= to; i++ {
		availablePorts.PushFront(strconv.Itoa(i))
	}
}

func Deploy() {
	log.Println("DEPLOYING NEW RUNNING SERVER")
	testing.mutex.Lock()
	server := fs.GetBaseFromServer(testing.servlet.Dir)
	testing.servlet.Kill()
	availablePorts.PushFront(testing.servlet.Port)
	testing.servlet = nil
	testing.mutex.Unlock()

	oldFolder := fs.GetBaseFromServer(running.servlet.Dir)
	err := newServer(server, typelib.RUNNING, &running)
	if err != nil {
		log.AddError(err).Debug("New server deployment")
	}
	oldFolders <- oldFolder
	return
}

func Restart(t typelib.ServerType) (err error) {
	switch t {
	case typelib.RUNNING:
		err = newServer(fs.GetBaseFromServer(running.servlet.Dir), t, &running)
	case typelib.TESTING:
		err = newServer(fs.GetBaseFromServer(testing.servlet.Dir), t, &testing)
	}
	return
}

func NewTesting(server string) (err error) {
	path, err := fs.CreateNewServerStructure(server)
	if err != nil {
		return
	}
	oldFolder := ""
	if testing.servlet != nil {
		oldFolder = fs.GetBaseFromServer(testing.servlet.Dir)
	}
	err = newServer(path, typelib.TESTING, &testing)
	if oldFolder != "" {
		oldFolders <- oldFolder
	}
	return
}

func ChekcReliability() {
	log.Println("reliabilityScore of testingServer compared to runningServer: ", testing.reliabilityScore(*running.servlet))
	if testing.reliabilityScore(*running.servlet) >= -0.25 {
		testing.once.Do(Deploy)
	}
}

func InitServers(workingDir string, of chan<- string) (err error) {
	oldFolders = of
	firstServerPath, err := fs.GetFirstServerDir(workingDir, typelib.RUNNING)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug(firstServerPath)
	err = newServer(firstServerPath, typelib.RUNNING, &running)
	if err != nil {
		return
	}

	firstTestServerPath, err := fs.GetFirstServerDir(workingDir, typelib.TESTING)
	if err != nil {
		return
	}
	log.Debug(firstTestServerPath)
	if firstServerPath != firstTestServerPath {
		err = newServer(firstTestServerPath, typelib.TESTING, &testing)
	}
	return
}
