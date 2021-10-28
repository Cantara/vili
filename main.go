package main

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
