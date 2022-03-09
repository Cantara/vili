package fslib

import (
	"bytes"
	"os"
	"testing"
)

func TestInterfaceDir(t *testing.T) {
	var d Dir
	dir, err := NewInMemDir("name")
	if err != nil {
		t.Error(err)
		return
	}
	d = &dir
	if d.Path() != "name" {
		t.Error("Name from interface is wrong")
	}
}

func validateFSBase(d dir, basePath string, shouldBeInMem bool, t *testing.T) {
	if !d.base.IsDir() {
		t.Errorf("Dir base was not dir")
		return
	}
	if d.inMem != shouldBeInMem {
		t.Errorf("Dir was in mem when it shouldn't have been")
		return
	}
	if d.base.Path() != basePath {
		t.Errorf("Dir base path was incorrect")
		return
	}
}

func TestFS(t *testing.T) {
	d1Path := "/"
	d, err := NewDir(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(d, d1Path, false, t)
}

func TestFSFromWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}
	d, err := NewDirFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(d, wd, false, t)
}

func TestInMemFS(t *testing.T) {
	d1Path := "/"
	d, err := NewInMemDir(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	validateFSBase(d, d1Path, true, t)
}

func TestFSNewInMemDir(t *testing.T) {
	d1Path := "/"
	d, err := NewInMemDir(d1Path)
	if err != nil {
		t.Error(err)
		return
	}
	d2Name := "testD2"
	dir, err := d.Mkdir(d2Name, 0755)
	if err != nil {
		t.Error(err)
		return
	}
	if dir.Path() != d1Path+d2Name {
		t.Error("Subfoler path is not correct from mkdir")
		return
	}
	/*
		if !dir.base.IsDir() {
			t.Error("Mkdir did not create dir")
			return
		}
		if !dir.inMem {
			t.Error("InMem FS did not create InMem dir with mkdir")
			return
		}
	*/
	files, err := d.ReadDir(".")
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
	d, err := NewDirFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	d2Name := "testD2"
	d.Remove(d2Name)
	files, err := d.ReadDir(".")
	if err != nil {
		t.Error(err)
		return
	}
	antFiles := len(files)
	dir, err := d.Mkdir(d2Name, 0755)
	if err != nil {
		t.Error(err)
		return
	}
	if dir.Path() != d.Path()+"/"+d2Name {
		t.Error("Subfoler path is not correct from mkdir", dir.Path(), d.Path()+"/"+d2Name)
		return
	}
	/*
		if !dir.base.IsDir() {
			t.Error("Mkdir did not create dir")
			return
		}
		if dir.inMem {
			t.Error("Not InMem FS did create InMem dir with mkdir")
			return
		}
	*/
	files, err = d.ReadDir(".")
	if err != nil {
		t.Error(err)
		return
	}
	if len(files) != antFiles+1 {
		t.Errorf("Wrong amount of files in dir, %d of %d", len(files), antFiles+1)
		return
	}
}

func testWriteCopyAndReadFile(d dir, t *testing.T) {
	f1Name, f2Name := "fsysF1", "fsysF2"
	d.Remove(f1Name)
	d.Remove(f2Name)

	f1, err := d.Create(f1Name)
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
	err = d.FindAndCopy(f1Name, f2Name)
	if err != nil {
		t.Error(err)
		return
	}
	read, err := d.ReadFile(f2Name)
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
	d, err := NewDirFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	testWriteCopyAndReadFile(d, t)
}

func TestFSInMemWriteCopyAndReadFile(t *testing.T) {
	d, err := NewInMemDir("/")
	if err != nil {
		t.Error(err)
		return
	}
	testWriteCopyAndReadFile(d, t)
}
