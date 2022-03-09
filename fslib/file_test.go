package fslib

import (
	"bytes"
	"os"
	"testing"
)

func TestInterfaceFile(t *testing.T) {
	var f File
	file, err := NewFileInMem("name", nil)
	if err != nil {
		t.Error(err)
		return
	}
	f = &file
	if f.Name() != "name" {
		t.Error("Name from interface is wrong")
	}
}

func TestDir(t *testing.T) {
	d1Path := "/"
	d1, err := NewDirFile(d1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if !d1.IsDir() {
		t.Errorf("New dir was not dir")
		return
	}
}

func TestFile(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFile(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if f1.IsDir() {
		t.Errorf("New file was dir")
		return
	}
}

func TestInMemDir(t *testing.T) {
	d1Path := "/"
	d1, err := NewDirInMem(d1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if !d1.inMem {
		t.Errorf("New dir was not inMem")
		return
	}
}

func TestInMemFile(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFileInMem(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if !f1.inMem {
		t.Errorf("New file was not inMem")
		return
	}
}

func TestInMemParentDir(t *testing.T) {
	d1Path := "/"
	d1, err := NewDirInMem(d1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	f1Path := "/f1"
	f1, err := NewFileInMem(f1Path, &d1)
	if err != nil {
		t.Error(err)
		return
	}
	if f1.dir != &d1 {
		t.Errorf("File one parent was not d1")
		return
	}
}

func TestParentDir(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}
	d1Path := dir + "/test"
	d1, err := NewDirFile(d1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	f1Path := d1Path + "/f1"
	f1, err := NewFile(f1Path, &d1)
	if err != nil {
		t.Error(err)
		return
	}
	f1.Remove()
	d1.Remove()
	err = d1.Mkdir(0755)
	if err != nil {
		t.Error(err)
		return
	}
	err = f1.Create()
	if err != nil {
		t.Error(err)
		return
	}
	if f1.dir != &d1 {
		t.Errorf("File one parent was not d1")
		return
	}
}

func TestFileWriteFileNotOpened(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFile(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	data := []byte("TEST DATA")
	_, err = f1.Write(data)
	if err != FileNotOpened {
		t.Errorf("Write without opening file didn't give FileNotOpened error")
		return
	}
}

func TestFileReadFileNotOpened(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFile(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	data := []byte("TEST DATA")
	_, err = f1.Read(data)
	if err != FileNotOpened {
		t.Errorf("Write without opening file didn't give FileNotOpened error")
		return
	}
}

func TestFileInMemWriteFileNotOpened(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFileInMem(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	data := []byte("TEST DATA")
	_, err = f1.Write(data)
	if err != FileNotOpened {
		t.Errorf("Write without opening file didn't give FileNotOpened error")
		return
	}
}

func TestFileInMemReadFileNotOpened(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFileInMem(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	data := []byte("TEST DATA")
	_, err = f1.Read(data)
	if err != FileNotOpened {
		t.Errorf("Write without opening file didn't give FileNotOpened error")
		return
	}
}

func testWriteAndRead(f file, t *testing.T) {
	f.Remove() //Removing test file without thinking of errors

	err := f.Create()
	if err != nil {
		t.Error(err)
		return
	}
	write := []byte("TEST DATA")
	n, err := f.Write(write)
	f.Close()
	if err != nil {
		t.Error(err)
		return
	}
	if n != len(write) {
		t.Errorf("Write did not write expected length of data %d of %d", n, len(write))
		return
	}
	err = f.Open()
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	read := make([]byte, len(write))
	n, err = f.Read(read)
	if err != nil {
		t.Error(err)
		return
	}
	if n != len(write) {
		t.Errorf("Read did not read expected length(%d of %d) of data, %s", n, len(write), read)
		return
	}
	if !bytes.Equal(read, write) {
		t.Errorf("Write and Read did not give the same data, %s != %s", read, write)
		return
	}
}

func TestFileWriteAndRead(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFile(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	testWriteAndRead(f1, t)
}

func TestFileInMemWriteAndRead(t *testing.T) {
	f1Path := "f1"
	f1, err := NewFileInMem(f1Path, nil)
	if err != nil {
		t.Error(err)
		return
	}
	testWriteAndRead(f1, t)
}
