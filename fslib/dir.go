package fslib

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/cantara/bragi"
)

type dir struct {
	base  File
	inMem bool
}

func NewDirFromWD() (d dir, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	d, err = NewDir(wd)
	return
}

func NewDir(path string) (d dir, err error) {
	base, err := NewDirFile(path, nil)
	if err != nil {
		return
	}
	d = dir{base: &base}
	return
}

func NewInMemDir(path string) (d dir, err error) {
	base, err := NewDirInMem(path, nil)
	d = dir{
		base:  &base,
		inMem: true,
	}
	return
}

func dirFromFile(f File, inMem bool) (d dir, err error) {
	if !f.IsDir() {
		err = FileNotDir
		return
	}
	d = dir{
		base:  f,
		inMem: inMem, //Need to fix this
	}
	return
}

func (d dir) Open(name string) (fOut fs.File, err error) {
	f, err := d.Find(name)
	if err != nil {
		return
	}
	err = f.Open()
	if err != nil {
		return
	}
	fOut = f
	return
}

func (d *dir) Remove(path string) (err error) {
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.Remove()
}

func (d *dir) RemoveAll(path string) (err error) {
	if path == "*" {
		var dirData []File
		dirData, err = d.base.Readdir()
		if err != nil {
			return
		}
		for i := range dirData {
			err = dirData[i].RemoveAll()
			if err != nil {
				return
			}
		}
		return
	}
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.RemoveAll()
}

func (d *dir) Create(name string) (f File, err error) {
	name = d.fixPath(name)
	dir, err := d.Find(filepath.Dir(name))
	if err != nil {
		return
	}
	f, err = dir.NewFile(name)
	if err != nil {
		return
	}
	err = f.Create()
	return
}

func (d *dir) Mkdir(name string, perm fs.FileMode) (dOut Dir, err error) {
	name = d.fixPath(name)
	dir, err := d.Find(filepath.Dir(name))
	if err != nil {
		return
	}
	f, err := dir.NewDir(name)
	if err != nil {
		return
	}
	err = f.Mkdir(perm)
	if err != nil {
		return
	}
	dTmp, err := dirFromFile(f, d.inMem)
	if err != nil {
		return
	}
	return &dTmp, nil
}

func (d dir) Stat(path string) (fileInfo fs.FileInfo, err error) {
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.Stat()
}

func (d dir) Cd(dir string) (dOut Dir, err error) {
	f, err := d.Find(dir)
	if err != nil {
		return
	}
	dTmp, err := dirFromFile(f, d.inMem) //Could be better for memory if new file was creted without parent
	if err != nil {
		return
	}
	return &dTmp, nil
}

func (d dir) Path() string {
	return d.base.Path()
}

func (d dir) BaseDir() File {
	return d.base
}

func (d dir) Exists(path string) bool {
	f, err := d.Find(path)
	if err != nil {
		return false
	}
	return f.Exists()
}

func (d dir) ReadDir(path string) ([]fs.DirEntry, error) {
	dir, err := d.Find(path)
	if err != nil {
		return nil, err
	}
	return dir.ReadDir()
}

func (d dir) Readdir(path string) ([]File, error) {
	dir, err := d.Find(path)
	if err != nil {
		return nil, err
	}
	return dir.Readdir()
}

func (d dir) ReadFile(name string) (out []byte, err error) {
	f, err := d.Find(name)
	if err != nil {
		return nil, err
	}
	err = f.Open()
	if err != nil {
		return
	}
	defer f.Close()
	out = make([]byte, f.Size())
	_, err = f.Read(out)
	return
}

func (d *dir) Copy(src File, dst string) error {
	if dst == "" {
		return fmt.Errorf("Dest is missing for copy file")
	}
	dst = d.fixPath(dst)
	if !d.Exists(filepath.Dir(dst)) {
		return fmt.Errorf("Destination dir does not exist when trying to copy file")
	}
	if d.Exists(dst) {
		return fmt.Errorf("Destination file does allready exist when trying to copy file")
	}

	if !src.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src.Path())
	}

	err := src.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	destination, err := d.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, src)
	return err
}

func (d *dir) FindAndCopy(src, dst string) error {
	srcFile, err := d.Find(src)
	if err != nil {
		return err
	}
	return d.Copy(srcFile, dst)
}

func (d *dir) Symlink(src File, dst string) error {
	if dst == "" {
		return fmt.Errorf("Dest is missing for copy file")
	}
	dst = d.fixPath(dst)
	if !d.Exists(filepath.Dir(dst)) {
		return fmt.Errorf("Destination dir does not exist when trying to copy file")
	}
	if d.Exists(dst) {
		return fmt.Errorf("Destination file does allready exist when trying to copy file")
	}

	return src.Symlink(dst)
}

func (d dir) Readlink(path string) (out string, err error) {
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.Readlink()
}

func (d dir) File() File {
	return d.base
}

func (d dir) Size() int64 {
	var total int64
	fs.WalkDir(d, d.Path(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.AddError(err).Debug("While reading dir to get size")
			return nil
		}
		if !d.IsDir() {
			inf, err := d.Info()
			if err != nil {
				log.AddError(err).Debug("While getting file info to get dir size")
				return nil
			}
			total += inf.Size()
		}
		return nil
	})
	return total
}

func (d dir) Find(path string) (out File, err error) {
	if !strings.HasPrefix(path, d.Path()) {
		path = fmt.Sprintf("%s/%s", d.Path(), path)
	}
	return d.find(filepath.Clean(path))
}

func (d dir) FindDir(path string) (out File, err error) {
	return d.Find(filepath.Dir(path))
}

func (d dir) find(path string) (out File, err error) { //Expects a clean path TODO Add tests
	if path == "." || d.Path() == path {
		return d.base, nil
	}
	//var dirData []File
	dirData, err := d.base.Readdir()
	for i := range dirData {
		if dirData[i].Path() == path {
			return dirData[i], nil
		}
		if !dirData[i].IsDir() {
			continue
		}
		//dTmp, err := d.Cd(dirData[i].Path())
		dTmp, err := dirFromFile(dirData[i], d.inMem) //Could be better for memory if new file was creted without parent
		if err != nil {
			return nil, err
		}
		out, err = dTmp.find(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if out != nil {
			return out, err
		}
	}
	if out == nil {
		err = fs.ErrNotExist
	}
	return
}

/*
func (fsys FS) find(path string) (out *file, err error) {
	return fsys.base.find(path)
}
*/

func (d dir) fixPath(path string) string {
	if !strings.HasPrefix(path, d.base.Path()) {
		path = fmt.Sprintf("%s/%s", d.base.Path(), path)
	}
	return filepath.Clean(path)
}

func (d dir) PrintTree() {
	fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		fmt.Println(path)
		return nil
	})
}

func (d dir) String() string {
	return d.Path()
}
