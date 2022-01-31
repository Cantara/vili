package fs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/typelib"
)

func stripJar(s string) string {
	if len(s) < 4 {
		return s
	}
	return s[:len(s)-4]
}

func GetFileFromPath(path string) string {
	pathParts := strings.Split(path, "/")
	return pathParts[len(pathParts)-1]
}

func getBaseFromInstance(instance string) string {
	path := strings.Split(instance, "/")
	return strings.Join(path[:len(path)-2], "/") //TODO handle if server is not long enugh aka correct
}

func GetBaseFromServer(server string) string {
	path := strings.Split(server, "/")
	return strings.Join(path[:len(path)-1], "/") //TODO handle if server is not long enugh aka correct
}

func CreateNewServerStructure(server string) (newFolder string, err error) { // This could do with some error handling instead of just panic
	newFolder = stripJar(server)
	err = os.Mkdir(newFolder, 0755)
	if err != nil {
		return
	}
	err = copyFile(server, fmt.Sprintf("%s/%s", newFolder, GetFileFromPath(server))) // os.Rename
	return
}

func copyFile(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("Source or dest is missing for copy file")
	}
	if !FileExists(src) {
		return fmt.Errorf("Source file does not exist when trying to copy file")
	}
	if FileExists(dst) {
		return fmt.Errorf("Destination file does allready exist when trying to copy file")
	}
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func CreateNewServerInstanceStructure(server string, t typelib.ServerType, port string) (newInstancePath string, err error) { // This could do with some error handling instead of just panic
	if strings.HasSuffix(server, "/") {
		server = server[:len(server)-1]
	}
	serverFile := fmt.Sprintf("%s/%s.jar", server, GetFileFromPath(server))
	if !FileExists(serverFile) {
		err = fmt.Errorf("Server file does not excist in server folder, thus unable to create now instance structure")
		return
	}
	newInstancePath = fmt.Sprintf("%s/%s_%s", server, time.Now().Format("2006-01-02_15.04.05"), t) //, numRestartsOfType(server, t)+1)
	err = os.Mkdir(newInstancePath, 0755)
	if err != nil {
		return
	}
	newFile := fmt.Sprintf("%s/current", server)
	os.Remove(newFile)
	os.Symlink(newInstancePath, newFile)
	err = os.Mkdir(newInstancePath+"/logs", 0755)
	if err != nil {
		return
	}
	err = os.Mkdir(newInstancePath+"/logs/json", 0755)
	if err != nil {
		return
	}
	newFile = fmt.Sprintf("%s/logs", server)
	os.Remove(newFile)
	os.Symlink(newInstancePath+"/logs", newFile)
	base := GetBaseFromServer(server)
	newFile = fmt.Sprintf("%s/logs_%s-%s", base, os.Getenv("identifier"), t)
	os.Remove(newFile)
	os.Symlink(newInstancePath+"/logs", newFile)
	newFilePath := fmt.Sprintf("%s/%s.jar", newInstancePath, os.Getenv("identifier"))
	err = os.Symlink(serverFile, newFilePath)
	if err != nil {
		return
	}
	err = copyPropertyFile(newInstancePath, port, t)
	if err != nil {

	}
	err = copyAuthorizationFile(newInstancePath)
	return
}

func SymlinkFolder(server string, t typelib.ServerType) error {
	newFile := fmt.Sprintf("%s-%s", os.Getenv("identifier"), t)
	os.Remove(newFile)
	return os.Symlink(server, newFile)
}

func GetFirstServerDir(wd string, t typelib.ServerType) (name string, err error) {
	fileName := fmt.Sprintf("%s/%s-%s", wd, os.Getenv("identifier"), t)
	if FileExists(fileName) { // Might change this to do it manualy and actually check if it is a dir and so on.
		name, err = os.Readlink(fileName)
		if err == nil {
			return
		}
		log.Println(err)
	}
	name, err = getNewestServerDir(wd, t)
	if err != nil {
		return
	}
	if name == "" {
		err = fmt.Errorf("No server of type %s found.", t)
		return
	}
	name = fmt.Sprintf("%s/%s", wd, name)
	return
}

func getNewestServerDir(wd string, t typelib.ServerType) (name string, err error) {
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
	if (nameDir == "" || (t == typelib.TESTING && timeFile.After(timeDir))) && nameFile != "" {
		nameDir = stripJar(nameFile)
		err = os.Mkdir(nameDir, 0755)
		if err != nil {
			return
		}
		err = copyFile(nameFile, fmt.Sprintf("%s/%s", nameDir, nameFile))
		if err != nil {
			return
		}
	}
	return nameDir, nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func copyPropertyFile(instance, port string, t typelib.ServerType) (err error) {
	propertiesFileName := os.Getenv("properties_file_name")
	if propertiesFileName == "" {
		return
	}
	fileIn, err := os.Open(fmt.Sprintf("%s/%s", getBaseFromInstance(instance), propertiesFileName))
	if err != nil {
		return
	}
	defer fileIn.Close()
	fileOut, err := os.Create(fmt.Sprintf("%s/%s", instance, propertiesFileName))
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

func copyAuthorizationFile(instance string) (err error) {
	fileName := os.Getenv("properties_file_name")
	fileName = "authorization.properties"
	if fileName == "" || !FileExists(fileName) {
		return
	}
	err = copyFile(fmt.Sprintf("%s/%s", getBaseFromInstance(instance), fileName), fmt.Sprintf("%s/%s", instance, fileName))
	return
}

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
