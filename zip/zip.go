package zip

import (
	"archive/zip"
	"compress/flate"
	"io"
	"io/ioutil"

	log "github.com/cantara/bragi"
	"github.com/cantara/vili/fslib"
)

type Zipper struct {
	Dir fslib.Dir
}

func (z Zipper) ZipDir(serverDir fslib.Dir) (err error) {
	log.Println("Achiving server ", serverDir)
	outFile, err := z.Dir.Create(serverDir.File().Name() + ".zip")
	if err != nil {
		return
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	w.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	err = addFiles(w, serverDir, "")
	if err != nil {
		return
	}

	// Make sure to check the error on Close.
	err = w.Close()
	if err != nil {
		return
	}
	err = serverDir.File().RemoveAll()
	return
}

func addFiles(w *zip.Writer, serverDir fslib.Dir, baseInZip string) (err error) {
	files, err := serverDir.ReadDir("*")
	if err != nil {
		return
	}

	for _, file := range files {
		log.Println("ziping: " + serverDir.Path() + file.Name())
		if !file.IsDir() {
			dat, err := ioutil.ReadFile(serverDir.Path() + file.Name())
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
			newBase, err := serverDir.Cd(file.Name())
			if err != nil {
				return err
			}
			err = addFiles(w, newBase, baseInZip+file.Name()+"/")
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
	return
}
