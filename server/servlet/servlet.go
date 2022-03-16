package servlet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fslib"
	"github.com/cantara/vili/tail"
)

type servlet struct {
	port     string
	dir      fslib.Dir
	errors   int64
	warnings int64
	breaking int64
	requests int64
	cmd      *exec.Cmd
	version  string
	ctx      context.Context
	once     sync.Once
	kill     func()
}

func (s *servlet) Kill() {
	s.kill()
}

func (s servlet) Dir() fslib.Dir {
	return s.dir
}

func (s servlet) Port() string {
	return s.port
}

func (s servlet) ReliabilityScore() int64 {
	return s.requests - s.breaking*100 - s.errors*10 - s.warnings
}

func (s *servlet) IncrementBreaking() {
	atomic.AddInt64(&s.breaking, 1)
}

func (s *servlet) IncrementErrors() {
	atomic.AddInt64(&s.errors, 1)
}

func (s *servlet) IncrementWarnings() {
	atomic.AddInt64(&s.warnings, 1)
}

func (s *servlet) IncrementRequests() {
	atomic.AddInt64(&s.requests, 1)
}

func (s *servlet) ResetTestData() {
	atomic.StoreInt64(&s.warnings, 0)
	atomic.StoreInt64(&s.errors, 0)
	atomic.StoreInt64(&s.breaking, 0)
	atomic.StoreInt64(&s.requests, 0)
}

func (s servlet) IsRunning() bool {
	return s.cmd.Process.Signal(syscall.Signal(0)) == nil
}

func NewServlet(servletDir fslib.Dir, port string) (s *servlet, err error) {
	stdOut, err := servletDir.Create("stdOut") //, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	stdErr, err := servletDir.Create("stdErr") //, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	server, err := servletDir.Find(os.Getenv("identifier") + ".jar")
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("java", "-jar", server.Path()) //fmt.Sprintf("%s/%s.jar", servletDir.Path(), os.Getenv("identifier")))
	if os.Getenv("properties_file_name") == "" {
		cmd = exec.Command("java", fmt.Sprintf("-D%s=%s", os.Getenv("port_identifier"), port), "-jar", server.Path()) //fmt.Sprintf("%s/%s.jar", servletDir.Path(), os.Getenv("identifier")))
	}
	cmd.Dir = servletDir.Path()
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	log.Debug(cmd)
	err = cmd.Start()
	if err != nil {
		return
	}
	pid, err := servletDir.Create("pid") //, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err == nil {
		fmt.Fprintln(pid, cmd.Process.Pid)
		pid.Close()
	}
	time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the service can start without getting any new request, this should not be needed
	s = &servlet{
		port: port,
		dir:  servletDir,
		cmd:  cmd,
		ctx:  ctx,
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
	go s.parseLogServer(ctx)
	return
}

type logData struct {
	Level string `json:"level"`
}

func (servlet *servlet) parseLogServer(ctx context.Context) {
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
