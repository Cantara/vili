package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fs"
	"github.com/cantara/vili/server"
	"github.com/cantara/vili/slack"
	"github.com/cantara/vili/typelib"
	"github.com/cantara/vili/zip"
	"github.com/joho/godotenv"
	"k8s.io/utils/inotify"
)

type endpointToVerify struct {
	oldResponse *http.Response
	request     *http.Request
}

type viliDashAction struct {
	Server string `json:"server"`
	Action string `json:"action"`
}

var endpoint string
var z zip.Zipper

func loadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}
}

func verifyConfig() error {
	if os.Getenv("scheme") != "http" && os.Getenv("scheme") != "https" {
		return fmt.Errorf("scheme needs to be either http or https") // This requirement could probably be removed, i think vili should be able to handle other schemes like file and so on
	}
	if os.Getenv("endpoint") == "" {
		return fmt.Errorf("No endpoint provided")
	}
	if !strings.Contains(os.Getenv("port_range"), "-") || strings.Contains(os.Getenv("port_range"), " ") {
		return fmt.Errorf("Portrange is not a range in the format of <number>-<number>")
	}
	if os.Getenv("identifier") == "" {
		return fmt.Errorf("No identifier provided")
	}
	if os.Getenv("port_identifier") == "" {
		return fmt.Errorf("No port identifier provided")
	}
	return nil
}

func main() {
	loadEnv()

	logDir := os.Getenv("log_dir")
	if logDir != "" {
		log.SetPrefix("vili")
		cloaser := log.SetOutputFolder(logDir)
		if cloaser == nil {
			log.Fatal("Unable to sett logdir")
		}
		defer cloaser()
		done := make(chan func())
		log.StartRotate(done)
		defer close(done)
	}
	err := verifyConfig()
	if err != nil {
		log.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	slack.Client = slack.NewClient(os.Getenv("app_icon"), os.Getenv("env_icon"), os.Getenv("env"), os.Getenv("identifier"))
	slack.Sendf(" :heart: Vili starting on host: %s", hostname)

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	z = zip.Zipper{
		OutDir: fmt.Sprintf("%s/%s", wd, "archive"),
	}
	if !fs.FileExists(z.OutDir) {
		err = os.Mkdir(z.OutDir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

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
	zipperChan := make(chan string, 1)
	fileSystem := os.DirFS(wd)
	go func() {
		for {
			oldFolder := <-zipperChan
			err = z.ZipDir(oldFolder)
			if err != nil {
				log.Println(err)
			}
			for fs.GetDirSize(fileSystem, "archive") > 1<<30 {
				go slack.Sendf("Archive too large, cleaning up on server: %s.", hostname)
				os.Remove(fs.GetOldestFile(fileSystem, "archive"))
			}
		}
	}()

	verifyChan := make(chan endpointToVerify, 10) // Arbitrary large number that hopefully will not block
	serv, err := server.Initialize(wd, zipperChan, from, to)
	if err != nil {
		slack.Sendf(":sos: @channel Uable to initialize vili on host %s.", hostname)
		log.AddError(err).Fatal("While inizalicing server")
	}
	defer serv.Kill()
	go slack.Sendf(" :white_check_mark: Vili startet initial services on host: %s, with running version %s.", hostname, serv.GetRunningVersion())

	go func() {
		for {
			etv := <-verifyChan
			if serv.HasTesting() {
				go func() {
					rNew, err := requestHandler(endpoint+":"+serv.GetPort(typelib.TESTING), etv.request, &serv, true)
					if err != nil {
						log.Println(err)
						return
					}
					defer rNew.Body.Close()
					err = verifyNewResponse(etv.oldResponse, rNew)
					if err != nil {
						serv.AddBreaking(typelib.TESTING)
					}
					serv.AddRequest(typelib.TESTING)
					serv.CheckReliability(hostname)
					if time.Minute*10 <= serv.TestingDuration() {
						serv.ResetTest()
					}
				}()
			}
		}
	}()

	watcher, err := inotify.NewWatcher()
	if err != nil {
		slack.Sendf(":sos: @channel Uable to fully start vili, couldn't start watcher %s.", hostname)
		log.Fatal(err)
	}
	defer watcher.Close()
	err = watcher.AddWatch(wd, inotify.InCreate)
	if err != nil {
		slack.Sendf(":sos: @channel Uable to fully start vili, couldn't add listner to watcher %s.", hostname)
		log.Fatal(err)
	}
	defer watcher.RemoveWatch(wd)
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				log.Println("event:", ev)
				path := strings.Split(ev.Name, "/")
				name := strings.ToLower(path[len(path)-1])
				identifier := strings.ToLower(os.Getenv("identifier"))
				if !strings.HasSuffix(name, ".jar") {
					continue
				}
				if !strings.HasPrefix(name, identifier) {
					continue
				}
				if name == identifier+".jar" {
					continue
				}
				time.Sleep(time.Second * 2) //Sleep an arbitrary amout of time so the file is done writing before we try to execute it
				go slack.Sendf(" :mailbox_with_mail: :clock12: New version found, downloaded and deployed, running version is: %s, starting to test version %s.", serv.GetRunningVersion(), ev.Name)
				serv.NewTesting(ev.Name)
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	if os.Getenv("manualcontrol") == "true" {
		servData := struct {
			Identity string `json:"identity"`
			Uid      string `json:"uid"`
			Ip       string `json:"ip"`
			RunningV string `json:"running_version"`
			TestingV string `json:"testing_version"`
		}{
			Identity: os.Getenv("identifier"),
			Ip:       "0.0.0.0",
			RunningV: "unknown",
			TestingV: "unknown",
		}
		go func() {
			viliDashBaseURI := "https://api-devtest.entraos.io/vili-dash"
			err = post(viliDashBaseURI+"/register/server", &servData, &servData)
			for err != nil {
				log.Info(err)
				time.Sleep(time.Second * 30)
				err = post(viliDashBaseURI+"/register/server", &servData, &servData)
			}
			for {
				time.Sleep(time.Minute)
				var vda viliDashAction
				err = get(viliDashBaseURI+"/action/"+os.Getenv("identifier")+"/"+servData.Uid, &vda)
				if err != nil {
					log.Println(err)
					continue
				}
				switch vda.Action {
				case "deploy":
					serv.Deploy()
				case "restart":
					serv.Restart(typelib.FromString(vda.Server))
				}
			}
		}()
	}

	s := &http.Server{
		Addr:           ":" + os.Getenv("port"),
		Handler:        http.HandlerFunc(reqHandler(&serv, verifyChan)),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Println(s.Addr + "/*")
	log.Fatal(s.ListenAndServe())
}

func get(uri string, out interface{}) (err error) {
	resp, err := http.Get(uri)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, out)
	return
}

func post(uri string, data interface{}, out interface{}) (err error) {
	jsonValue, _ := json.Marshal(data)
	buf := bytes.NewReader(jsonValue)
	resp, err := http.Post(uri, "application/json", buf)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, out)
	return
}

func reqHandler(serv *server.Server, etv chan<- endpointToVerify) http.HandlerFunc { //TODO Remove dependencie on pointer
	return func(w http.ResponseWriter, r *http.Request) {
		rOld, err := requestHandler(endpoint+":"+serv.GetPort(typelib.RUNNING), r, serv, false)
		if err != nil {
			log.AddError(err).Info("While proxying to running")
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
		rOld.Body.Close()
		serv.AddRequest(typelib.RUNNING)

		etv <- endpointToVerify{
			oldResponse: rOld,
			request:     r,
		}
	}
}

func requestHandler(host string, r *http.Request, serv *server.Server, test bool) (*http.Response, error) { // Return response
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
		if !strings.HasSuffix(r.URL.String(), "health") {
			log.Printf("%s %s %s", prefix, resp.Status, r.URL)
		}
	} else {
		if !test {
			if !serv.IsRunning(typelib.RUNNING) {
				serv.Restart(typelib.RUNNING)
			}
		} else {
			if !serv.IsRunning(typelib.TESTING) {
				serv.Restart(typelib.TESTING)
			}
		}
	}
	return resp, e
}

func verifyNewResponse(r, t *http.Response) error { // Take inn responses
	if r.StatusCode == t.StatusCode {
		return nil
	}
	if r.StatusCode != http.StatusNotFound && t.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Missing endpoint")
	}
	return nil
}

/*

Should the system use scripts to handle starting and stopping or should it just use the exev lib? Will using the the exec lib remove the connectivity between this program and the server itself? Does it matter if this program and the server is tightly coupeled. If this program crashed the server will be unreacheble anyways.

Conclusion, the coupeling between this program and the server does not matter. If this program crashes then the server will be unreachable anyways.



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
