package main

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func zipDir(server string) (err error) {
	log.Println("Achiving server ", server)
	outFile, err := os.Create(fmt.Sprintf("%s/archive/%s.zip", getBaseFromServer(server), getFileFromPath(server)))
	if err != nil {
		return
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	w.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	err = addFiles(w, server+"/", "")
	if err != nil {
		return
	}

	// Make sure to check the error on Close.
	err = w.Close()
	if err != nil {
		return
	}
	err = os.RemoveAll(server)
	return
}

func addFiles(w *zip.Writer, basePath, baseInZip string) (err error) {
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return
	}

	for _, file := range files {
		fmt.Println(basePath + file.Name())
		if !file.IsDir() {
			dat, err := ioutil.ReadFile(basePath + file.Name())
			if err != nil {
				log.Println(err)
				continue
			}

			f, err := w.Create(baseInZip + file.Name())
			if err != nil {
				log.Println(err)
				continue
			}
			_, err = f.Write(dat)
			if err != nil {
				log.Println(err)
				continue
			}
		} else if file.IsDir() {
			newBase := basePath + file.Name() + "/"
			err = addFiles(w, newBase, baseInZip+file.Name()+"/")
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
	return
}

func getBaseFromServer(server string) string {
	path := strings.Split(server, "/")
	return strings.Join(path[:len(path)-1], "/") //TODO handle if server is not long enugh aka correct
}
