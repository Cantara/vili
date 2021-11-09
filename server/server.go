package server

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fs"
	"github.com/cantara/vili/tail"
	"github.com/cantara/vili/typelib"
)

var running ServerHandler
var testing ServerHandler
var availablePorts *list.List
var oldFolders chan<- string

type ServerHandler struct {
	server     *Server
	mutex      sync.Mutex
	serverType typelib.ServerType
}

type Server struct {
	port       string
	errors     uint64
	warnings   uint64
	breaking   uint64
	requests   uint64
	mesureFrom time.Time
	server     *exec.Cmd
	version    string
	ctx        context.Context
	once       sync.Once
	kill       func()
}

func (s Server) reliabilityScore(compServ *Server) float64 {
	if time.Now().Sub(s.mesureFrom) < time.Minute*1 {
		return -1
	}
	return s.internalReliabilityScore() - compServ.internalReliabilityScore()
}

func (s Server) internalReliabilityScore() float64 {
	return math.Log2(float64(s.requests) - float64(s.breaking*100+s.errors*10+s.warnings))
}

func newServer(path string, t typelib.ServerType, serverHandler *ServerHandler) (err error) {
	port := getPort()
	newPath, err := fs.CreateNewServerInstanceStructure(path, t, port)
	if err != nil {
		availablePorts.PushFront(port)
		return
	}

	s := startNewServer(newPath, port)
	if s == nil {
		availablePorts.PushFront(port)
		return
	}
	s.port = port

	serverHandler.mutex.Lock()
	var oldServer *Server
	oldServer, serverHandler.server = serverHandler.server, s
	serverHandler.mutex.Unlock()

	err = fs.SymlinkFolder(path, t)
	if oldServer != nil {
		oldServer.kill()
		availablePorts.PushFront(oldServer.port)
	}
	return
}

func startNewServer(serverFolder, port string) *Server {
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
	cmd := exec.Command("java", "-jar", fmt.Sprintf("%s/%s.jar", serverFolder, os.Getenv("identifier")))
	if os.Getenv("properties_file_name") == "" {
		cmd = exec.Command("java", fmt.Sprintf("-D%s=%s", os.Getenv("port_identifier"), port), "-jar", fmt.Sprintf("%s/%s.jar", serverFolder, os.Getenv("identifier")))
	}
	cmd.Dir = serverFolder
	log.Debug(cmd)
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
	server := &Server{
		server: cmd,
		ctx:    ctx,
		kill: func() {
			err := cmd.Process.Kill() //.Signal(syscall.SIGTERM)
			if err != nil {
				log.Println(err)
			}
			err = cmd.Wait()
			if err != nil {
				log.Println(err)
			}
			cancel()
			stdOut.Close()
			stdErr.Close()
		},
	}
	go parseLogServer(server, ctx)
	return server
}

type logData struct {
	Level string `json:"level"`
}

func parseLogServer(server *Server, ctx context.Context) {
	lineChan, err := tail.File(fmt.Sprintf("%s/logs/json/%s.log", server.server.Dir, os.Getenv("identifier")), ctx)
	if err != nil {
		log.Println(err) //TODO look into what can be done here
		return
	}
	for {
		select {
		case line, ok := <-lineChan:
			if !ok {
				log.Println("LineChan closed closing log parser")
				return
			}
			if server.mesureFrom.IsZero() {
				continue
			}
			var data logData
			err := json.Unmarshal(line, &data)
			if err != nil {
				log.Println(err)
				continue
			}
			switch data.Level {
			case "WARN":
				server.warnings++
			case "ERROR":
				server.errors++
			}
		case <-ctx.Done():
			log.Println("Closing log parser")
			return
		}
	}
}

func GetPort(t typelib.ServerType) string {
	switch t {
	case typelib.RUNNING:
		return running.server.port
	case typelib.TESTING:
		return testing.server.port
	}
	return ""
}

func AddBreaking(t typelib.ServerType) {
	if !Messuring() {
		return
	}
	switch t {
	case typelib.RUNNING:
		atomic.AddUint64(&running.server.breaking, 1)
	case typelib.TESTING:
		atomic.AddUint64(&testing.server.breaking, 1)
	}
}

func AddRequest(t typelib.ServerType) {
	if !Messuring() {
		return
	}
	switch t {
	case typelib.RUNNING:
		atomic.AddUint64(&running.server.requests, 1)
	case typelib.TESTING:
		atomic.AddUint64(&testing.server.requests, 1)
	}
}

func HasTesting() bool {
	testing.mutex.Lock()
	defer testing.mutex.Unlock()
	return testing.server != nil
}

func Messuring() bool {
	if testing.server == nil {
		return false
	}
	testing.mutex.Lock()
	defer testing.mutex.Unlock()
	return !testing.server.mesureFrom.IsZero()
}

func ResetTest() { //TODO: Make better
	if testing.server == nil {
		return
	}
	if !Messuring() {
		testing.server.mesureFrom = time.Now()
		testing.server.warnings = 0
		testing.server.errors = 0
		testing.server.breaking = 0
		testing.server.requests = 0
		running.server.mesureFrom = time.Now()
		running.server.warnings = 0
		running.server.errors = 0
		running.server.breaking = 0
		running.server.requests = 0
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
	server := fs.GetBaseFromServer(testing.server.server.Dir)
	testing.server.kill()
	availablePorts.PushFront(testing.server.port)
	testing.server = nil
	testing.mutex.Unlock()

	oldFolder := fs.GetBaseFromServer(running.server.server.Dir)
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
		err = newServer(fs.GetBaseFromServer(running.server.server.Dir), t, &running)
	case typelib.TESTING:
		err = newServer(fs.GetBaseFromServer(testing.server.server.Dir), t, &testing)
	}
	return
}

func NewTesting(server string) (err error) {
	path, err := fs.CreateNewServerStructure(server)
	if err != nil {
		return
	}
	oldFolder := ""
	if testing.server != nil {
		oldFolder = fs.GetBaseFromServer(testing.server.server.Dir)
	}
	err = newServer(path, typelib.TESTING, &testing)
	if oldFolder != "" {
		oldFolders <- oldFolder
	}
	return
}

func ChekcReliability() {
	log.Println("reliabilityScore of testingServer compared to runningServer: ", testing.server.reliabilityScore(running.server))
	if testing.server.reliabilityScore(running.server) >= -0.25 {
		testing.server.once.Do(Deploy)
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
