package fslib

import (
	"fmt"
	"io"
	stdFS "io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Dir struct {
	base  *file
	inMem bool
}

func NewDirFromWD() (d Dir, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	d, err = NewDir(wd)
	return
}

func NewDir(dir string) (d Dir, err error) {
	base, err := DirFile(dir, nil)
	if err != nil {
		return
	}
	d = Dir{base: &base}
	return
}

func NewInMemDir(dir string) (d Dir, err error) {
	base, err := DirInMem(dir, nil)
	d = Dir{
		base:  &base,
		inMem: true,
	}
	return
}

func dirFromFile(f *file) (d Dir, err error) {
	if !f.IsDir() {
		err = FileNotDir
		return
	}
	d = Dir{
		base:  f,
		inMem: f.inMem,
	}
	return
}

func (d Dir) Open(name string) (f *file, err error) {
	f, err = d.Find(name)
	if err != nil {
		return
	}
	err = f.Open()
	return
}

func (d *Dir) Remove(name string) (err error) {
	f, err := d.Find(name)
	if err != nil {
		return
	}
	err = f.Remove()
	return
}

func (d *Dir) Create(name string) (f *file, err error) {
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

func (d *Dir) Mkdir(name string, perm stdFS.FileMode) (dOut Dir, err error) {
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
	return dirFromFile(f)
}

func (d Dir) Stat(path string) (fileInfo stdFS.FileInfo, err error) {
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.Stat()
}

func (d Dir) Cd(dir string) (dOut Dir, err error) {
	f, err := d.Find(dir)
	if err != nil {
		return
	}
	return dirFromFile(f) //Could be better for memory if new file was creted without parent
}

func (d Dir) Path() string {
	return d.base.Path()
}

func (d Dir) BaseDir() *file {
	return d.base
}

func (d Dir) Exists(path string) bool {
	f, err := d.Find(path)
	if err != nil {
		return false
	}
	return f.Exists()
}

func (d Dir) ReadDir(path string) ([]stdFS.DirEntry, error) {
	dir, err := d.Find(path)
	if err != nil {
		return nil, err
	}
	return dir.ReadDir()
}

func (d Dir) Readdir(path string) ([]stdFS.FileInfo, error) {
	dir, err := d.Find(path)
	if err != nil {
		return nil, err
	}
	return dir.Readdir()
}

func (d Dir) ReadFile(name string) (out []byte, err error) {
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

func (d *Dir) Copy(src *file, dst string) error {
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

func (d *Dir) FindAndCopy(src, dst string) error {
	srcFile, err := d.Find(src)
	if err != nil {
		return err
	}
	return d.Copy(srcFile, dst)
}

func (d *Dir) Symlink(src file, dst string) error {
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

	err := src.Symlink(dst)
	if err != nil {
		return err
	}
	dir, err := d.FindDir(dst)
	if err != nil {
		return err
	}
	linkFile, err := d.Find(src.path)
	if err != nil {
		return err
	}
	dir.symlinkFile(linkFile)
	return nil
}

func (d Dir) Readlink(path string) (out string, err error) {
	f, err := d.Find(path)
	if err != nil {
		return
	}
	return f.Readlink()
}

func (d Dir) Find(path string) (out *file, err error) {
	if !strings.HasPrefix(path, d.base.Path()) {
		path = fmt.Sprintf("%s/%s", d.base.Path(), path)
	}
	return d.base.find(filepath.Clean(path))
}

func (d Dir) FindDir(path string) (out *file, err error) {
	return d.Find(filepath.Dir(path))
}

/*
func (fsys FS) find(path string) (out *file, err error) {
	return fsys.base.find(path)
}
*/

func (d Dir) fixPath(path string) string {
	if !strings.HasPrefix(path, d.base.path) {
		path = fmt.Sprintf("%s/%s", d.base.path, path)
	}
	return filepath.Clean(path)
}
