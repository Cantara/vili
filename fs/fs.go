package fs

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fslib"
	"github.com/cantara/vili/typelib"
)

func stripJar(s string) string {
	if len(s) < 4 {
		return s
	}
	return s[:len(s)-4]
}

var baseDir fslib.Dir

func Initialize(dir fslib.Dir) {
	baseDir = dir
}

func CreateNewServerStructure(server string) (newDir fslib.Dir, err error) {
	newDir, err = baseDir.Mkdir(stripJar(server), 0755)
	if err != nil {
		return
	}
	serverFile, err := baseDir.Find(server)
	if err != nil {
		return
	}

	err = baseDir.Copy(serverFile, fmt.Sprintf("%s/%s", newDir.Path(), serverFile.Name()))
	return
}

func CreateNewServerInstanceStructure(serverDir fslib.Dir, t typelib.ServerType, port string) (instanceDir fslib.Dir, err error) {
	outerServerFile, err := fslib.NewFile(serverDir.File().Name()+".jar", nil)
	if err != nil {
		return
	}
	serverFile, err := serverDir.Find(outerServerFile.Name()) //Not totaly sure what to do with server here / what format i want server on etc
	if err != nil {
		err = fmt.Errorf("Server file does not excist in server folder, thus unable to create new instance structure: err(%v) %s", err, outerServerFile.Name())
		return
	}
	newInstancePath := fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02_15.04.05"), t) //, numRestartsOfType(server, t)+1)
	instanceDir, err = serverDir.Mkdir(newInstancePath, 0755)
	if err != nil {
		return
	}

	serverDir.Remove("current")
	serverDir.Symlink(instanceDir.BaseDir(), "current")

	logs, err := instanceDir.Mkdir("logs", 0755) //There is something here i don't like
	if err != nil {
		return
	}
	_, err = logs.Mkdir("json", 0755)
	if err != nil {
		return
	}

	serverDir.Remove("logs")
	serverDir.Symlink(logs.BaseDir(), "logs")

	//TODO move to another function i think
	baseLogs := fmt.Sprintf("logs_%s-%s", os.Getenv("identifier"), t)
	baseDir.Remove(baseLogs)
	baseDir.Symlink(logs.BaseDir(), baseLogs)
	baseVersion := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	baseDir.Remove(baseVersion)
	baseDir.Symlink(serverDir.BaseDir(), baseVersion)
	instanceExecPath := fmt.Sprintf("%s/%s.jar", instanceDir.Path(), os.Getenv("identifier"))
	err = serverDir.Symlink(serverFile, instanceExecPath)
	if err != nil {
		return
	}
	err = copyPropertyFile(instanceDir, port, t)
	if err != nil {
		return
	}
	authName := "authorization.properties"
	baseDir.FindAndCopy(authName, instanceDir.Path()+"/"+authName) //Could change copy function to add filename if none is given
	return
}

func GetFirstServerDir(t typelib.ServerType) (serverDir fslib.Dir, err error) {
	fileName := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	if baseDir.Exists(fileName) {
		name, err := baseDir.Readlink(fileName)
		if err == nil {
			serverDir, err = baseDir.Cd(name)
			return serverDir, err
		}
		log.Println(err)
	}
	name, err := getNewestServerDir(t)
	if err != nil {
		return
	}
	/*if name == "" {
		err = fmt.Errorf("No server of type %s found.", t)
		return
	}*/
	//serverDir, err = baseDir.Cd(name.Path())
	serverDir = name
	return
}

func getNewestServerDir(t typelib.ServerType) (serverDir fslib.Dir, err error) {
	files, err := baseDir.Readdir(".")
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
			if nameDir != "" && isSemanticNewer("*.*.*", toVersion(file.Name()), toVersion(nameDir)) { //timeDir.After(file.ModTime()) {
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
	if (nameDir == "" || (t == typelib.TESTING && timeFile.After(timeDir))) && nameFile != "" {
		serverDir, err = CreateNewServerStructure(nameFile)
	} else {
		serverDir, err = baseDir.Cd(nameDir)
	}
	return
}

func copyPropertyFile(instanceDir fslib.Dir, port string, t typelib.ServerType) (err error) {
	propertiesFileName := os.Getenv("properties_file_name")
	if propertiesFileName == "" {
		return
	}
	fileIn, err := baseDir.Open(propertiesFileName)
	if err != nil {
		return
	}
	defer fileIn.Close()
	fileOut, err := instanceDir.Create(propertiesFileName)
	if err != nil {
		return
	}
	defer fileOut.Close()

	scanner := bufio.NewScanner(fileIn)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	overritenPort := false
	fileOut.WriteString("# This is a copied and modified propertie file.\n# Modifications are done by Vili\n")
	fileOut.WriteString(fmt.Sprintf("vili.test=%t\n", t == typelib.TESTING))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, os.Getenv("port_identifier")+"=") {
			fileOut.WriteString(fmt.Sprintf("%s=%s\n", os.Getenv("port_identifier"), port))
			overritenPort = true
			continue
		}
		fileOut.WriteString(line + "\n")
	}
	if !overritenPort {
		fileOut.WriteString(fmt.Sprintf("%s=%s\n", os.Getenv("port_identifier"), port))
	}
	return
}

/*
func copyAuthorizationFile(instance string) (err error) {
	fileName := os.Getenv("properties_file_name")
	fileName = "authorization.properties"
	if fileName == "" || !FileExists(fileName) {
		return
	}
	err = copyFile(fmt.Sprintf("%s/%s", getBaseFromInstance(instance), fileName), fmt.Sprintf("%s/%s", instance, fileName))
	return
}
*/

func GetOldestFile(fsys fslib.Dir) string {
	oldestPath := ""
	oldest := time.Now()
	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.AddError(err).Info("While reading dir to get oldest file")
			return nil
		}
		if !d.IsDir() {
			inf, err := d.Info()
			if err != nil {
				log.AddError(err).Info("While getting file info to get oldest file")
				return nil
			}
			if oldest.Before(inf.ModTime()) {
				return nil
			}
			oldestPath = path
			oldest = inf.ModTime()
		}
		return nil
	})
	return oldestPath
}

func isSemanticNewer(filter string, p1, p2 string) bool {
	log.Printf("Testing %s vs %s with filter %s\n", p1, p2, filter)
	numLevels := 3
	levels := strings.Split(filter, ".")
	if len(levels) != numLevels {
		log.Fatal("Invalid semantic filter, expecting *.*.*")
	}
	p1v := strings.Split(p1, ".")
	if len(p1v) != numLevels {
		log.Fatal("Invalid semantic version for arg 2, expecting *.*.*")
	}
	p2v := strings.Split(p2, ".")
	if len(p2v) != numLevels {
		log.Fatal("Invalid semantic version for arg 3, expecting *.*.*")
	}
	for i := 0; i < numLevels; i++ {
		if levels[i] == "*" {
			v1, err := strconv.Atoi(p1v[i])
			if err != nil {
				log.Fatal(err)
			}
			v2, err := strconv.Atoi(p2v[i])
			if err != nil {
				log.Fatal(err)
			}
			if v1 < v2 {
				log.Printf("v1 < v2 = %d < %d", v1, v2)
				return true
			}
			if v1 > v2 {
				log.Printf("v1 > v2 = %d > %d", v1, v2)
				return false
			}
		}
	}
	return false
}

func toVersion(fileName string) string {
	fileName = strings.ReplaceAll(fileName, os.Getenv("identifier"), "")
	fileName = strings.ReplaceAll(fileName, ".jar", "")
	fileName = strings.TrimLeft(fileName, "-")
	fileName = strings.Split(fileName, "-")[0]
	return fileName
}
