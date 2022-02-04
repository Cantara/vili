package fslib

import (
	"errors"
	"fmt"
	"io"
	stdFS "io/fs"
	"os"
	"path/filepath"
	"time"
)

type file struct {
	opened  bool
	inMem   bool
	dir     *file
	data    []byte
	dirData []*file
	size    int64
	path    string
	modTime time.Time
	mode    stdFS.FileMode
	isDir   bool
	osFile  *os.File
	cur     int
}

var (
	InvalidPath   = errors.New("invalid path")
	FileNotOpened = errors.New("file not opened")
	FileOpened    = errors.New("file opened")
	FileNotDir    = errors.New("file is not dir")
)

func File(path string, dir *file) (f file, err error) {
	f = file{
		path: filepath.Clean(path),
		dir:  dir,
	}
	if dir != nil && dir.path != filepath.Clean(f.Dir()) {
		err = InvalidPath
	}
	return
}

func Dir(path string, dir *file) (f file, err error) {
	f, err = File(path, dir)
	if err != nil {
		return
	}
	f.isDir = true
	return
}

func FileInMem(path string, dir *file) (f file, err error) {
	f, err = File(path, dir)
	if err != nil {
		return
	}
	f.inMem = true
	return
}

func DirInMem(path string, dir *file) (f file, err error) {
	f, err = Dir(path, dir)
	if err != nil {
		return
	}
	f.inMem = true
	return
}

func FileFromFileInfo(path string, fileInfo stdFS.FileInfo, dir *file) (f file, err error) {
	f, err = File(path, dir)
	if err != nil {
		return
	}
	f.isDir = fileInfo.IsDir()
	f.mode = fileInfo.Mode()
	f.size = fileInfo.Size()
	f.modTime = fileInfo.ModTime()
	return
}

func (f *file) Open() (err error) {
	if f.opened {
		return FileOpened
	}
	if f.inMem {
		f.cur = 0
		f.opened = true
		return nil
	}

	_, err = f.Stat()
	if err != nil {
		return
	}
	f.osFile, err = os.Open(f.path)
	if err != nil {
		return
	}
	if !f.IsDir() {
		f.opened = true
		return
	}

	files, err := f.osFile.Readdir(0) //Using this to not get f.opened not opened error //Might get away with Readdir instead of ReadDir
	if err != nil {
		fmt.Println(files)
		return
	}
	for i := range files {
		/*
			var fileInfo stdFS.FileInfo
			fileInfo, err = files[i].Info()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				fmt.Println("Wierd error: ", err.Error())
				return
			}
			fffi, err := FileFromFileInfo(fmt.Sprintf("%s/%s", f.path, fileInfo.Name()), fileInfo, f)
		*/
		fffi, err := FileFromFileInfo(fmt.Sprintf("%s/%s", f.path, files[i].Name()), files[i], f)
		if err != nil {
			continue //Could be that i should return here
		}
		f.dirData = append(f.dirData, &fffi)
	}
	f.opened = true
	return
}

func (f *file) Create() (err error) {
	if f.opened {
		return FileOpened
	}
	if f.inMem {
		f.opened = true
		return nil
	}
	if f.Exists() {
		return stdFS.ErrExist
	}
	f.osFile, err = os.Create(f.path)
	if err != nil {
		return
	}
	f.opened = true
	return
}

func (f *file) NewFile(path string) (fout *file, err error) {
	if !f.IsDir() {
		err = FileNotDir
		return
	}
	var ft file
	if f.inMem {
		ft, err = FileInMem(path, f)
		if err != nil {
			return
		}
		fout = &ft
		f.dirData = append(f.dirData, fout)
	} else {
		ft, err = File(path, f)
		if err != nil {
			return
		}
		fout = &ft
	}
	return
}

func (f *file) NewDir(path string) (fout *file, err error) {
	if !f.IsDir() {
		err = FileNotDir
		return
	}
	var ft file
	if f.inMem {
		ft, err = DirInMem(path, f)
		if err != nil {
			return
		}
		fout = &ft
		f.dirData = append(f.dirData, fout)
	} else {
		ft, err = Dir(path, f)
		if err != nil {
			return
		}
		fout = &ft
	}
	return
}

func (f *file) Mkdir(perm stdFS.FileMode) (err error) {
	if !f.IsDir() {
		return FileNotDir
	}
	if f.inMem {
		return
	}
	f.mode = perm
	return os.Mkdir(f.path, f.mode)
}

func (f *file) Remove() (err error) {
	if f.opened {
		err = f.Close()
		if err != nil {
			return
		}
	}
	if f.inMem {
		if f.IsDir() {
			f.dirData = nil
		} else {
			f.data = []byte{}
		}
		return
	}

	return os.Remove(f.path)
}

func (f file) Exists() bool {
	_, err := f.Stat()
	return !errors.Is(err, os.ErrNotExist)
}

func (f *file) Stat() (fileInfoOut stdFS.FileInfo, err error) {
	if f.inMem {
		return f, nil
	}
	fileInfo, err := os.Stat(f.path)
	if err != nil {
		return
	}
	f.size = fileInfo.Size()
	f.mode = fileInfo.Mode()
	f.modTime = fileInfo.ModTime()
	f.isDir = fileInfo.IsDir()
	return f, nil
}

func (f *file) Read(out []byte) (length int, err error) {
	if !f.opened {
		return 0, FileNotOpened
	}
	if f.IsDir() {
		return
	}

	if f.inMem {
		if len(f.data) == f.cur {
			err = io.EOF
			return
		}
		length = copy(out, f.data[f.cur:])
		f.cur += length
		return
	}
	return f.osFile.Read(out)
}

func (f file) ReadDir() ([]stdFS.DirEntry, error) {
	if f.opened {
		return nil, FileOpened
	}
	if !f.inMem {
		err := f.Open()
		if err != nil {
			return nil, err
		}
	}
	out := make([]stdFS.DirEntry, len(f.dirData))
	for i := range f.dirData {
		out[i] = f.dirData[i]
	}
	if !f.inMem {
		err := f.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (f *file) Close() (err error) {
	if f.inMem { // Could delete data before returning, depending on intended behavior
		f.opened = false
		return
	}
	err = f.osFile.Close()
	if err != nil {
		return
	}
	f.opened = false
	f.osFile = nil
	return
}

func (f file) Name() string {
	_, name := filepath.Split(f.path)
	return name
}

func (f file) Dir() (dir string) {
	dir, _ = filepath.Split(f.path)
	return
}

func (f file) Size() int64 {
	if f.inMem {
		return int64(len(f.data))
	}

	return f.size
}

func (f file) Mode() stdFS.FileMode {
	return f.mode
}

func (f file) Type() stdFS.FileMode {
	return f.mode
}

func (f file) ModTime() time.Time {
	return f.modTime
}

func (f file) IsDir() bool {
	return f.isDir
}

func (f file) Sys() interface{} {
	return nil //TODO: Not implemented
}

func (f file) Info() (stdFS.FileInfo, error) {
	return f, nil
}

func (f *file) Write(b []byte) (n int, err error) {
	if !f.opened {
		return 0, FileNotOpened
	}
	if f.IsDir() {
		return
	}
	if f.inMem { //This is getting ugly
		data := make([]byte, f.cur+len(b))
		if f.cur != 0 {
			copy(data[:f.cur], f.data)
		}
		n := copy(data[f.cur:], b)
		f.cur += n
		f.data = data
		return n, nil
	}
	return f.osFile.Write(b)
}

func (f *file) find(path string) (out *file, err error) { //Expects a clean path TODO Add tests
	if path == "." || f.path == path {
		return f, nil
	}
	if !f.IsDir() {
		return
	}
	if !f.opened {
		err = f.Open()
		if err != nil {
			return
		}
		defer f.Close()
	}
	for i := range f.dirData {
		if f.dirData[i].path == path {
			return f.dirData[i], nil
		}
		out, err = f.dirData[i].find(path)
		if out != nil {
			return
		}
	}
	if out == nil {
		err = stdFS.ErrNotExist
	}
	return
}
