package main

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"k8s.io/utils/inotify"
)

type endpointToVerify struct {
	oldResponse *http.Response
	request     *http.Request
}

type serve struct { // TODO rename
	port     string
	errors   int
	warnings int
	requests int
	server   *exec.Cmd
	ctx      context.Context
	kill     func()
}

func (s serve) reliabilityScore(compServ *serve) float64 {
	return s.internalReliabilityScore() - compServ.internalReliabilityScore()
}

func (s serve) internalReliabilityScore() float64 {
	return float64(s.requests) / float64(s.errors*10+s.warnings+1)
}

var runningServer *serve
var testingServer *serve
var availablePorts *list.List
var endpoint string

func loadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}
}

func main() {
	loadEnv()
	f, err := os.OpenFile(os.Getenv("log_file"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	log.SetOutput(f)
	endpoint = os.Getenv("endpoint")
	r := os.Getenv("port_range")
	ports := strings.Split(r, "-")
	from, err := strconv.Atoi(ports[0])
	if err != nil {
		log.Fatal(err)
	}
	to, err := strconv.Atoi(ports[1])
	if err != nil {
		log.Fatal(err)
	}
	availablePorts = list.New()
	for i := from; i <= to; i++ {
		availablePorts.PushFront(strconv.Itoa(i))
	}

	verifyChan := make(chan endpointToVerify, 10) // Arbitrary large number that hopefully will not block
	go func() {
		for {
			etv := <-verifyChan
			go func() {
				if testingServer != nil {
					rNew, err := requestHandler(endpoint+":"+testingServer.port, etv.request, true)
					if err != nil {
						log.Println(err)
						return
					}
					err = verifyNewResponse(etv.oldResponse, rNew)
					if err != nil {
						log.Println(err)
						return
					}
					testingServer.requests++
					log.Println("reliabilityScore of testingServer compared to runningServer: ", testingServer.reliabilityScore(runningServer))
					if testingServer.reliabilityScore(runningServer) > 1 {
						deploy(&runningServer, &testingServer)
					}
				}
			}()
		}
	}()

	watcher, err := inotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.AddWatch(wd, inotify.InCreate)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.RemoveWatch(wd)
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				log.Println("event:", ev)
				if !strings.HasSuffix(ev.Name, ".jar") {
					continue
				}
				time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the file is done writing before we try to execute it
				path, err := createNewServerStructure(ev.Name)
				if err != nil {
					log.Println(err)
					continue
				}
				err = newServer(path, "test", &testingServer)
				if err != nil {
					log.Println(err)
					continue
				}
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	firstServerPath, err := getFirstServerDir(wd, "running")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(firstServerPath)
	err = newServer(firstServerPath, "running", &runningServer)
	if err != nil {
		log.Fatal(err)
	}

	firstTestServerPath, err := getFirstServerDir(wd, "test")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(firstTestServerPath)
	if firstServerPath != firstTestServerPath {
		err = newServer(firstTestServerPath, "test", &testingServer)
		if err != nil {
			log.Fatal(err)
		}
	}

	s := &http.Server{
		Addr:           ":" + os.Getenv("port"),
		Handler:        http.HandlerFunc(reqHandler(verifyChan)),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Println(s.Addr + "/*")
	log.Fatal(s.ListenAndServe())
}

func reqHandler(etv chan<- endpointToVerify) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rOld, err := requestHandler(endpoint+":"+runningServer.port, r, false)
		if err != nil {
			log.Println(err)
			return
		}
		for key, vals := range rOld.Header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		w.WriteHeader(rOld.StatusCode)
		for key, vals := range rOld.Trailer {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		io.Copy(w, rOld.Body)
		runningServer.requests++
		//fmt.Fprintln(w, rOld)

		etv <- endpointToVerify{
			oldResponse: rOld,
			request:     r,
		}
	}
}

func requestHandler(host string, r *http.Request, test bool) (*http.Response, error) { // Return response
	r.URL.Scheme = os.Getenv("scheme")
	r.URL.Host = host
	var body io.ReadCloser
	if r.GetBody != nil {
		body, _ = r.GetBody()
	}
	req := &http.Request{
		Method: r.Method,
		URL:    r.URL, //strings.Replace(*r.URL, strings.Split(*r.URL, "/")[0], endpoint),
		Body:   body,
		//		ContentLenght:    r.ContentLenght,
		TransferEncoding: r.TransferEncoding,
		Close:            true,
		Host:             r.Host, // replace r.Host with host if you want to change the host in the request not the url
		Form:             r.Form,
		PostForm:         r.PostForm,
		MultipartForm:    r.MultipartForm,
		Trailer:          r.Trailer,
	}
	resp, e := (&http.Client{}).Do(req)
	if e == nil {
		prefix := "[DEP]"
		if test {
			prefix = "[TEST]"
		}
		log.Printf("%s %s %s", prefix, resp.Status, r.URL)
	}
	return resp, e
}

func verifyNewResponse(r1, r2 *http.Response) error { // Take inn resonses
	if r1.StatusCode == r2.StatusCode {
		return nil
	}
	return fmt.Errorf("Not implemented verification")
}

func deploy(running, testing **serve) (err error) {
	path := strings.Split((*testing).server.Dir, "/")
	err = newServer(strings.Join(path[:len(path)-1], "/"), "running", testing)
	//*testing.kill()
	//availablePorts.PushFront(*testing.port)
	tmp := *running
	*running = *testing
	*testing = nil

	log.Println("KILLING OLD RUNNING SERVER")
	tmp.kill()
	availablePorts.PushFront(tmp.port)
	return
}

func newServer(path, t string, server **serve) (err error) {
	newPath, err := createNewServerInstanceStructure(path, t)
	if err != nil {
		return
	}
	log.Println("New path ", newPath)

	tmp := *server
	*server = startNewServer(newPath)

	err = symlinkFolder(path, t)
	if err != nil {
		return
	}
	//stop tmp
	if tmp != nil {
		log.Println("KILLING SERVER")
		tmp.kill()
		availablePorts.PushFront(tmp.port)
	}
	return
}

func createNewServerStructure(server string) (newFolder string, err error) { // This could do with some error handling instead of just panic
	//path := strings.Split(server, "/")
	newFolder = server[:len(server)-4]
	err = os.Mkdir(newFolder, 0755)
	if err != nil {
		return
	}
	//newFilePath = fmt.Sprintf("%s/%s", newFolder, path[len(path)-1])
	//err = os.Symlink(server, newFilePath)
	return
}

func createNewServerInstanceStructure(server, t string) (newInstancePath string, err error) { // This could do with some error handling instead of just panic
	// path := strings.Split(server, "/")
	// serverName := fmt.Sprintf("%s.jar", path[len(path)-1])
	newInstancePath = fmt.Sprintf("%s/%s-%d", server, t, numRestartsOfType(server, t)+1)
	err = os.Mkdir(newInstancePath, 0755)
	if err != nil {
		return
	}
	err = os.Mkdir(newInstancePath+"/logs", 0755)
	if err != nil {
		return
	}
	err = os.Mkdir(newInstancePath+"/logs/json", 0755)
	if err != nil {
		return
	}
	newFilePath := fmt.Sprintf("%s/%s.jar", newInstancePath, os.Getenv("identifier"))
	err = os.Symlink(server+".jar", newFilePath)
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
	port := getPort()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "java", "-Dserver.port="+port, "-jar", fmt.Sprintf("%s/%s.jar", serverFolder, os.Getenv("identifier")))
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
		port:   port,
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

func numRestartsOfType(dir, t string) (num int) {
	log.Println("COUNTING in di", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		if !strings.HasPrefix(file.Name(), t) {
			continue
		}
		num++
	}
	return
}

func symlinkFolder(server, t string) error {
	newFile := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	os.Remove(newFile)
	return os.Symlink(server, newFile)
}

func getFirstServerDir(wd, t string) (name string, err error) {
	fileName := fmt.Sprintf("%s/%s-%s", wd, os.Getenv("identifier"), t)
	if fileExists(fileName) { // Might change this to do it manualy and actually check if it is a dir and so on.
		path, err := os.Readlink(fileName)
		if err == nil {
			return path, nil
		}
		log.Println(err)
	}
	name, err = getNewestServerDir(wd, t)
	if err != nil {
		return
	}
	name = fmt.Sprintf("%s/%s", wd, name)
	log.Println("Server dir name: ", name)

	return //name, symlinkFolder(name, t)
}

func getNewestServerDir(wd, t string) (name string, err error) {
	files, err := ioutil.ReadDir(wd)
	if err != nil {
		return
	}
	timeDir := time.Unix(0, 0)
	timeFile := time.Unix(0, 0)
	nameDir, nameFile := "", ""
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), os.Getenv("identifier")) {
			continue
		}
		if file.Name() == os.Getenv("identifier")+".jar" {
			continue
		}
		if file.IsDir() {
			if timeDir.After(file.ModTime()) {
				continue
			}
			timeDir = file.ModTime()
			nameDir = file.Name()
			continue
		}
		if !strings.HasSuffix(file.Name(), ".jar") {
			continue
		}
		if timeFile.After(file.ModTime()) {
			continue
		}
		timeFile = file.ModTime()
		nameFile = file.Name()
	}
	if (nameDir == "" || (t == "test" && timeFile.After(timeDir))) && nameFile != "" {
		nameDir = nameFile[:len(nameFile)-4]
		err = os.Mkdir(nameDir, 0755)
		if err != nil {
			return
		}
	}
	return nameDir, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func getPort() string {
	port := availablePorts.Front()
	availablePorts.Remove(port)
	return port.Value.(string)
}

type logData struct {
	Level string `json:"level"`
}

func parseLogServer(server *serve, ctx context.Context) {
	lineChan, err := tailFile(fmt.Sprintf("%s/logs/json/%s.log", server.server.Dir, os.Getenv("identifier")), ctx)
	if err != nil {
		log.Println(err) //TODO look into what can be done here
		return
	}
	for {
		select {
		case line := <-lineChan:
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
			break
		}
	}
}

// This might be a better implementation?
/*func createNewServerStructure(server) (name string, err error) { // This could do with some error handling instead of just panic
	path := strings.Split(server, "/")
	name := path[len(path)-1][:len(path[len(path)-1])-4]
	//newInstancePath = fmt.Sprintf("%s-%d", serverPath, numRestartsOfType(serverPath, t)+1)
	err = os.Mkdir(name, 0755)
	//if err != nil {
	//	return
	//}
	//newFilePath := fmt.Sprintf("%s/%s.jar", newInstancePath, os.Getenv("identifier"))
	//err = os.Symlink(server, newFilePath)
	return
}*/

/* func tailFile(path string) (err error) {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		return
	}
	err = watcher.AddWatch(path, inotify.InCreate)
	if err != nil {
		return
	}
	go func() {
		defer watcher.RemoveWatch(path)
		for {
			select {
			case ev := <-watcher.Event:
				log.Println("event:", ev)
				if !strings.HasSuffix(ev.Name, ".jar") {
					continue
				}
				time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the file is done writing before we try to execute it
				path, err := createNewServerStructure(ev.Name)
				if err != nil {
					log.Println(err)
					continue
				}
				err = newServer(path, "test", &testingServer)
				if err != nil {
					log.Println(err)
					continue
				}
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()
}*/

/*func pullNewServer(script string) { // return the error instead
	cmd := exec.Command(script)
	err := cmd.Run()
	if err != nil {
		log.Printf("ERROR: Updating server\n %v\n", err)
	}
}


func getEndpoints() { // In tottos oppinion this is dead

}*/

/*

Should the system use scripts to handle starting and stopping or should it just use the exev lib? Will using the the exec lib remove the connectivity between this program and the server itself? Does it matter if this program and the server is tightly coupeled. If this program crashed the server will be unreacheble anyways.

Conclusion, the coupeling between this programm and the server does not matter. If this program crashes then the server will be unreachable anyways.



Should this program handle the downloading of new software or should it invoke a script. It looks like the script used today to downloade is quite large and this might be an optimal solution since it allready works and has alot of extra checks and functionality based on that. On the other hand, It could be better if this program were responsible for getting the new programs aswell so it could have more fine controll over naming and other things like that. That might make this a more stable and or simpler program. However that would couple this program close to more than one opperation making it difficult for others to just downloade and use it.

Conclusion, this needs to be further investigated. Thus this is not a problem i am looking at now.

New folder for process and not only for version
Notere pid for kjørende, gjerne i samme mappe som egen fil
Egen mappe for om det er test eller running
Symlink ifra base folder inn i running folder
Symlink for både running og test

Copy jar vil inn i versjons mappe også link til den

Look for exceptions in some file
*/
