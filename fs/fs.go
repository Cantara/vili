package fs

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
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

var baseDir fslib.FS

func CreateNewServerStructure(server string) (newFolder fslib.FS, err error) {
	newFolder, err = baseDir.Mkdir(stripJar(server), 0755)
	if err != nil {
		return
	}
	serverFile, err := baseDir.Find(server)
	if err != nil {
		return
	}

	err = baseDir.Copy(serverFile, fmt.Sprintf("%s/%s", newFolder.Path(), serverFile.Name()))
	return
}
func CreateNewServerInstanceStructure(serverDir fslib.FS, server string, t typelib.ServerType, port string) (newInstancePath string, err error) {
	outerServerFile, err := fslib.File(server, nil)
	if err != nil {
		return
	}
	serverFile, err := serverDir.Find(outerServerFile.Name()) //Not totaly sure what to do with server here / what format i want server on etc
	if err != nil {
		err = fmt.Errorf("Server file does not excist in server folder, thus unable to create now instance structure: err(%v)", err)
		return
	}
	newInstancePath = fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02_15.04.05"), t) //, numRestartsOfType(server, t)+1)
	instanceDir, err := serverDir.Mkdir(newInstancePath, 0755)
	if err != nil {
		return
	}

	//Symlink support needed in FSlib
	//newFile := fmt.Sprintf("%s/current", server)
	//os.Remove(newFile)
	//os.Symlink(newInstancePath, newFile)
	serverDir.Remove("current")
	serverDir.Symlink(*instanceDir.BaseDir(), "current")

	logs, err := instanceDir.Mkdir("logs", 0755) //There is something here i don't like
	if err != nil {
		return
	}
	_, err = logs.Mkdir("json", 0755)
	if err != nil {
		return
	}

	//Symlink support needed in FSlib
	//newFile = fmt.Sprintf("%s/logs", server)
	//os.Remove(newFile)
	serverDir.Remove("logs")
	serverDir.Symlink(*logs.BaseDir(), "logs")

	//TODO move to another function i think
	//base := GetBaseFromServer(server)
	baseLogs := fmt.Sprintf("logs_%s-%s", os.Getenv("identifier"), t)
	//os.Remove(newFile)
	//os.Symlink(newInstancePath+"/logs", newFile)
	baseDir.Remove(baseLogs)
	baseDir.Symlink(*logs.BaseDir(), baseLogs)
	instanceExecPath := fmt.Sprintf("%s/%s.jar", instanceDir.Path(), os.Getenv("identifier"))
	err = serverDir.Symlink(*serverFile, instanceExecPath)
	if err != nil {
		return
	}
	err = copyPropertyFile(&instanceDir, port, t)
	if err != nil {
		return
	}
	authName := "authorization.properties"
	baseDir.FindAndCopy(authName, instanceDir.Path()+"/"+authName) //Could change copy function to add filename if none is given
	return
}

/*
func SymlinkFolder(server string, t typelib.ServerType) error {
	newFile := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	os.Remove(newFile)
	return os.Symlink(server, newFile)
}
*/

func GetFirstServerDir(t typelib.ServerType) (serverDir fslib.FS, err error) {
	fileName := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	if baseDir.Exists(fileName) { // Might change this to do it manualy and actually check if it is a dir and so on.
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
	serverDir, err = baseDir.Cd(name.Path())
	return
}

func getNewestServerDir(t typelib.ServerType) (serverDir fslib.FS, err error) {
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
	if (nameDir == "" || (t == typelib.TESTING && timeFile.After(timeDir))) && nameFile != "" {
		serverDir, err = CreateNewServerStructure(nameFile)
	} else {
		serverDir, err = baseDir.Cd(nameDir)
	}
	return
}

func copyPropertyFile(instanceFS *fslib.FS, port string, t typelib.ServerType) (err error) {
	propertiesFileName := os.Getenv("properties_file_name")
	if propertiesFileName == "" {
		return
	}
	fileIn, err := baseDir.Open(propertiesFileName)
	if err != nil {
		return
	}
	defer fileIn.Close()
	fileOut, err := instanceFS.Create(propertiesFileName)
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

func GetOldestFile(fsys fs.FS, dir string) string {
	oldestPath := ""
	oldest := time.Now()
	fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
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

func GetDirSize(fsys fs.FS, dir string) int64 {
	var total int64
	fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.AddError(err).Info("While reading dir to get size")
			return nil
		}
		if !d.IsDir() {
			inf, err := d.Info()
			if err != nil {
				log.AddError(err).Info("While getting file info to get dir size")
				return nil
			}
			total += inf.Size()
		}
		return nil
	})
	return total
}
