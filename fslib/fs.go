package fslib

import (
	"fmt"
	"io"
	stdFS "io/fs"
	"os"
	"path/filepath"
	"strings"
)

type FS struct {
	base  *file
	inMem bool
}

func NewFSFromWD() (fsys FS, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	fsys, err = NewFS(wd)
	return
}

func NewFS(dir string) (fsys FS, err error) {
	base, err := Dir(dir, nil)
	fsys = FS{base: &base}
	return
}

func NewInMemFS(dir string) (fsys FS, err error) {
	base, err := DirInMem(dir, nil)
	fsys = FS{
		base:  &base,
		inMem: true,
	}
	return
}

func NewFSFromFile(f *file) FS {
	return FS{
		base:  f,
		inMem: f.inMem,
	}
}

func (fsys FS) Open(name string) (f *file, err error) {
	name = fsys.fixPath(name)
	f, err = fsys.find(name)
	if err != nil {
		return
	}
	err = f.Open()
	return
}

func (fsys *FS) Remove(name string) (err error) {
	name = fsys.fixPath(name)
	f, err := fsys.find(name)
	if err != nil {
		return
	}
	err = f.Remove()
	return
}

func (fsys *FS) Create(name string) (f *file, err error) {
	name = fsys.fixPath(name)
	dir, err := fsys.find(filepath.Dir(name))
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

func (fsys *FS) Mkdir(name string, perm stdFS.FileMode) (f *file, err error) {
	name = fsys.fixPath(name)
	dir, err := fsys.find(filepath.Dir(name))
	if err != nil {
		return
	}
	f, err = dir.NewDir(name)
	if err != nil {
		return
	}
	err = f.Mkdir(perm)
	return
}

func (fsys FS) Stat(path string) (fileInfo stdFS.FileInfo, err error) {
	path = fsys.fixPath(path)
	f, err := fsys.find(path)
	if err != nil {
		return
	}
	return f.Stat()
}

func (fsys FS) Cd(dir string) (fsysOut FS, err error) {
	dir = fsys.fixPath(dir)
	f, err := fsys.find(dir)
	if err != nil {
		return
	}
	fsysOut = NewFSFromFile(f) //Could be better for memory if new file was creted without parent
	return
}

func (fsys FS) Exists(path string) bool {
	path = fsys.fixPath(path)
	f, err := fsys.find(path)
	if err != nil {
		return false
	}
	return f.Exists()
}

func (fsys FS) ReadDir(path string) ([]stdFS.DirEntry, error) {
	path = fsys.fixPath(path)
	dir, err := fsys.find(path)
	if err != nil {
		return nil, err
	}
	return dir.ReadDir()
}

func (fsys FS) ReadFile(name string) (out []byte, err error) {
	name = fsys.fixPath(name)
	f, err := fsys.find(name)
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

func (fsys *FS) Copy(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("Source or dest is missing for copy file")
	}
	src = fsys.fixPath(src)
	dst = fsys.fixPath(dst)
	if !fsys.Exists(src) {
		return fmt.Errorf("Source file does not exist when trying to copy file")
	}
	if !fsys.Exists(filepath.Dir(dst)) {
		return fmt.Errorf("Destination dir does not exist when trying to copy file")
	}
	if fsys.Exists(dst) {
		return fmt.Errorf("Destination file does allready exist when trying to copy file")
	}
	sourceFileStat, err := fsys.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := fsys.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := fsys.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func (fsys FS) find(path string) (out *file, err error) {
	return fsys.base.find(path)
}

func (fsys FS) fixPath(path string) string {
	if !strings.HasPrefix(path, fsys.base.path) {
		path = fmt.Sprintf("%s/%s", fsys.base.path, path)
	}
	return filepath.Clean(path)
}
