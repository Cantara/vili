package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

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
