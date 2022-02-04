package fslib

import (
	"bytes"
	"os"
	"testing"
)

func validateFSBase(fsys FS, basePath string, shouldBeInMem bool, t *testing.T) {
	if !fsys.base.IsDir() {
		t.Errorf("Fsys base was not dir")
		return
	}
	if fsys.inMem != shouldBeInMem {
		t.Errorf("Fsys was in mem when it shouldn't have been")
		return
	}
	if fsys.base.path != basePath {
		t.Errorf("Fsys base path was incorrect")
		return
	}
}

func TestFS(t *testing.T) {
	d1Path := "/"
	fsys, err := NewFS(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(fsys, d1Path, false, t)
}

func TestFSFromWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}
	fsys, err := NewFSFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(fsys, wd, false, t)
}

func TestInMemFS(t *testing.T) {
	d1Path := "/"
	fsys, err := NewInMemFS(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(fsys, d1Path, true, t)
}

func TestFSNewInMemDir(t *testing.T) {
	d1Path := "/"
	fsys, err := NewInMemFS(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	d2Name := "testD2"
	dir, err := fsys.Mkdir(d2Name, 0755)
	if err != nil {
		t.Error(err)
		return
	}
	if dir.path != d1Path+d2Name {
		t.Error("Subfoler path is not correct from mkdir")
		return
	}
	if !dir.IsDir() {
		t.Error("Mkdir did not create dir")
		return
	}
	if !dir.inMem {
		t.Error("InMem FS did not create InMem dir with mkdir")
		return
	}
	files, err := fsys.ReadDir(".")
	if err != nil {
		t.Error(err)
		return
	}
	if len(files) != 1 {
		t.Errorf("Wrong amount of files in dir, %d", len(files))
		return
	}
}

func TestFSNewDir(t *testing.T) {
	fsys, err := NewFSFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	d2Name := "testD2"
	fsys.Remove(d2Name)
	files, err := fsys.ReadDir(".")
	if err != nil {
		t.Error(err)
		return
	}
	antFiles := len(files)
	dir, err := fsys.Mkdir(d2Name, 0755)
	if err != nil {
		t.Error(err)
		return
	}
	if dir.path != fsys.base.path+"/"+d2Name {
		t.Error("Subfoler path is not correct from mkdir", dir.path, fsys.base.path+"/"+d2Name)
		return
	}
	if !dir.IsDir() {
		t.Error("Mkdir did not create dir")
		return
	}
	if dir.inMem {
		t.Error("Not InMem FS did create InMem dir with mkdir")
		return
	}
	files, err = fsys.ReadDir(".")
	if err != nil {
		t.Error(err)
		return
	}
	if len(files) != antFiles+1 {
		t.Errorf("Wrong amount of files in dir, %d of %d", len(files), antFiles+1)
		return
	}
}

func testWriteCopyAndReadFile(fsys FS, t *testing.T) {
	f1Name, f2Name := "fsysF1", "fsysF2"
	fsys.Remove(f1Name)
	fsys.Remove(f2Name)

	f1, err := fsys.Create(f1Name)
	if err != nil {
		t.Error(err)
		return
	}
	write := []byte("TEST DATA")
	n, err := f1.Write(write)
	f1.Close()
	if err != nil {
		t.Error(err)
		return
	}
	if n != len(write) {
		t.Errorf("Write did not write expected length of data, %d of %d", n, len(write))
		return
	}
	err = fsys.Copy(f1Name, f2Name)
	if err != nil {
		t.Error(err)
		return
	}
	read, err := fsys.ReadFile(f2Name)
	if err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(read, write) {
		t.Errorf("Write and Read did not give the same data, %s != %s", read, write)
		return
	}
}

func TestFSWriteCopyAndReadFile(t *testing.T) {
	fsys, err := NewFSFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	testWriteCopyAndReadFile(fsys, t)
}

func TestFSInMemWriteCopyAndReadFile(t *testing.T) {
	fsys, err := NewInMemFS("/")
	if err != nil {
		t.Error(err)
		return
	}
	testWriteCopyAndReadFile(fsys, t)
}
