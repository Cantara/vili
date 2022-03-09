package fs

import (
	"fmt"
	"os"
	"testing"

	"github.com/cantara/vili/fslib"
	"github.com/cantara/vili/typelib"
)

func TestCreateServerStructureWithoutServerFile(t *testing.T) {
	err := os.Setenv("identifier", "something")
	if err != nil {
		t.Error(err)
		return
	}
	bDir, err := fslib.NewInMemDir("/")
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}

	_, err = CreateNewServerStructure("/something.jar")
	if err == nil {
		t.Error("No error when trying to create a new server without a runnable file")
		return
	}
	return
}

func TestCreateNewServerInstanceStructure(t *testing.T) {
	err := os.Setenv("identifier", "something")
	if err != nil {
		t.Error(err)
		return
	}
	bDir, err := fslib.NewInMemDir("/")
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}
	serverDir, err := baseDir.Mkdir("something", 0755)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = CreateNewServerInstanceStructure(serverDir, typelib.RUNNING, "8080")
	if err == nil {
		t.Error("No error when trying to create a new instance without a runnable file")
		return
	}
	return
}

func setupFullTestEnv() (identifier, localPropFileName string, err error) {
	identifier = "something"
	localPropFileName = "local_override.properties"
	err = os.Setenv("identifier", identifier)
	if err != nil {
		return
	}
	err = os.Setenv("properties_file_name", localPropFileName)
	return
}

func TestOneFSInMem(t *testing.T) {
	var err error
	bDir, err := fslib.NewInMemDir("/")
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}
	testFullOneServer(t)
}

func TestOneFSInWD(t *testing.T) {
	var err error
	bDir, err := fslib.NewDirFromWD()
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}
	testFullOneServer(t)
}

func TestFullFSInMem(t *testing.T) {
	var err error
	bDir, err := fslib.NewInMemDir("/")
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}
	server, serverDir, err := testFullOneServer(t)
	if err != nil {
		t.Error(err)
		return
	}
	testFullSecoundServer(server, serverDir, t)
}

func TestFullFSInWD(t *testing.T) {
	var err error
	bDir, err := fslib.NewDirFromWD()
	baseDir = &bDir
	if err != nil {
		t.Error(err)
		return
	}
	server, serverDir, err := testFullOneServer(t)
	if err != nil {
		t.Error(err)
		return
	}
	testFullSecoundServer(server, serverDir, t)
}

func testFullOneServer(t *testing.T) (server fslib.File, serverDir fslib.Dir, err error) {
	baseDir.RemoveAll("testDir")
	baseTestDir, err := baseDir.Mkdir("testDir", 0755)
	if err != nil {
		t.Error(err)
		return
	}
	baseDir = baseTestDir
	identifier, localPropFileName, err := setupFullTestEnv()
	if err != nil {
		t.Error(err)
		return
	}
	server, err = baseDir.Create(identifier + ".jar")
	if err != nil {
		t.Error(err)
		return
	}
	_, err = server.WriteString("Some string to give the file data")
	if err != nil {
		t.Error(err)
		return
	}
	server.Close()
	localProp, err := baseDir.Create(localPropFileName)
	if err != nil {
		t.Error(err)
		return
	}
	localProp.Close()
	authFileName := "authorization.properties"
	auth, err := baseDir.Create(authFileName)
	if err != nil {
		t.Error(err)
		return
	}
	auth.Close()

	serverDir, err = CreateNewServerStructure(server.Path())
	if err != nil {
		t.Error(err)
		return
	}
	if serverDir.Path() != baseDir.Path()+"/"+identifier {
		t.Error("Server dir path is incorect", serverDir.Path(), baseDir.Path()+identifier)
		return
	}
	if !baseDir.Exists(fmt.Sprintf("/%[1]s/%[1]s.jar", identifier)) {
		t.Error("Server structure is not correct")
		return
	}
	instanceDir, err := CreateNewServerInstanceStructure(serverDir, typelib.TESTING, "8080")
	if err != nil {
		t.Error(err)
		return
	}
	//instanceDir.PrintTree()
	//baseDir.PrintTree() TODO: Figure out why there is a 'loop' here
	if !instanceDir.Exists("logs") {
		t.Error("Log dir missing in instance dir")
		return
	}
	if !instanceDir.Exists("logs/json") {
		t.Error("Json log dir missing in instance dir")
		return
	}
	if !instanceDir.Exists(localPropFileName) {
		t.Error("Local properties file missing in instance dir")
		return
	}
	if !instanceDir.Exists(authFileName) {
		t.Error("Auth properties file missing in instance dir")
		return
	}
	return
}

func testFullSecoundServer(server fslib.File, serverDir fslib.Dir, t *testing.T) (err error) {
	return //TODO implement
}
