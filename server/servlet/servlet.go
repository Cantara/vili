package servlet

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/tail"
)

type Servlet struct {
	Port     string
	Dir      string
	errors   uint64
	warnings uint64
	breaking uint64
	requests uint64
	cmd      *exec.Cmd
	version  string
	ctx      context.Context
	once     sync.Once
	Kill     func()
}

func (s Servlet) ReliabilityScore() float64 {
	return math.Log2(float64(s.requests) - float64(s.breaking*100+s.errors*10+s.warnings))
}

func (s *Servlet) IncrementBreaking() {
	atomic.AddUint64(&s.breaking, 1)
}

func (s *Servlet) IncrementErrors() {
	atomic.AddUint64(&s.errors, 1)
}

func (s *Servlet) IncrementWarnings() {
	atomic.AddUint64(&s.warnings, 1)
}

func (s *Servlet) IncrementRequests() {
	atomic.AddUint64(&s.requests, 1)
}

func (s *Servlet) ResetTestData() {
	atomic.StoreUint64(&s.warnings, 0)
	atomic.StoreUint64(&s.errors, 0)
	atomic.StoreUint64(&s.breaking, 0)
	atomic.StoreUint64(&s.requests, 0)
}

func (s Servlet) IsRunning() bool {
	return s.cmd.Process.Signal(syscall.Signal(0)) == nil
}

func NewServlet(serverFolder, port string) (servlet Servlet, err error) {
	stdOut, err := os.OpenFile(serverFolder+"/stdOut", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	stdErr, err := os.OpenFile(serverFolder+"/stdErr", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
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
		return
	}
	pid, err := os.OpenFile(serverFolder+"/pid", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err == nil {
		fmt.Fprintln(pid, cmd.Process.Pid)
		pid.Close()
	}
	time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the service can start without getting any new request, this should not be needed
	servlet = Servlet{
		Port: port,
		Dir:  serverFolder,
		cmd:  cmd,
		ctx:  ctx,
		Kill: func() {
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
	go parseLogServer(servlet, ctx)
	return
}

type logData struct {
	Level string `json:"level"`
}

func parseLogServer(servlet Servlet, ctx context.Context) {
	lineChan, err := tail.File(fmt.Sprintf("%s/logs/json/%s.log", servlet.cmd.Dir, os.Getenv("identifier")), ctx)
	if err != nil {
		log.AddError(err).Error("While trying to tail log file") //TODO look into what can be done here
		return
	}
	for {
		select {
		case line, ok := <-lineChan:
			if !ok {
				log.Println("LineChan closed closing log parser")
				return
			}
			//TODO Should this check if we are messuring or just count as normal all the time?
			var data logData
			err := json.Unmarshal(line, &data)
			if err != nil {
				log.Println(err)
				continue
			}
			switch data.Level {
			case "WARN":
				servlet.IncrementWarnings()
			case "ERROR":
				servlet.IncrementErrors()
			}
		case <-ctx.Done():
			log.Println("Closing log parser")
			return
		}
	}
}
