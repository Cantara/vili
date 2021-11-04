package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/cantara/bragi"
)

func stripJar(s string) string {
	if len(s) < 4 {
		return s
	}
	return s[:len(s)-4]
}

func getFileFromPath(path string) string {
	pathParts := strings.Split(path, "/")
	return pathParts[len(pathParts)-1]
}

func getBaseFromInstance(instance string) string {
	path := strings.Split(instance, "/")
	return strings.Join(path[:len(path)-2], "/") //TODO handle if server is not long enugh aka correct
}

func createNewServerStructure(server string) (newFolder string, err error) { // This could do with some error handling instead of just panic
	newFolder = stripJar(server)
	err = os.Mkdir(newFolder, 0755)
	if err != nil {
		return
	}
	err = os.Rename(server, fmt.Sprintf("%s/%s", newFolder, getFileFromPath(server)))
	return
}

func createNewServerInstanceStructure(server, t, port string) (newInstancePath string, err error) { // This could do with some error handling instead of just panic
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
	if t == "running" {
		base := getBaseFromServer(server)
		newFile = fmt.Sprintf("%s/logs_%s", base, os.Getenv("identifier"))
		os.Remove(newFile)
		os.Symlink(newInstancePath+"/logs", newFile)
	}
	newFilePath := fmt.Sprintf("%s/%s.jar", newInstancePath, os.Getenv("identifier"))
	err = os.Symlink(fmt.Sprintf("%s/%s.jar", server, getFileFromPath(server)), newFilePath)
	if err != nil {
		return
	}
	err = copyPropertyFile(newInstancePath, port)
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
		nameDir = stripJar(nameFile)
		err = os.Mkdir(nameDir, 0755)
		if err != nil {
			return
		}
		err = os.Rename(nameFile, fmt.Sprintf("%s/%s", nameDir, nameFile))
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

func copyPropertyFile(instance, port string) (err error) {
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

func getBaseFromServer(server string) string {
	path := strings.Split(server, "/")
	return strings.Join(path[:len(path)-1], "/") //TODO handle if server is not long enugh aka correct
}
