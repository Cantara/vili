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
	baseDir, err = fslib.NewInMemDir("/")
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
	baseDir, err = fslib.NewInMemDir("/")
	if err != nil {
		t.Error(err)
		return
	}
	serverDir, err := baseDir.Mkdir("something", 0755)
	if err != nil {
		t.Error(err)
		return
	}

	_, err = CreateNewServerInstanceStructure(serverDir, "/something.jar", typelib.RUNNING, "8080")
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

func TestFullFSInMem(t *testing.T) {
	identifier, localPropFileName, err := setupFullTestEnv()
	if err != nil {
		t.Error(err)
		return
	}
	baseDir, err = fslib.NewInMemDir("/")
	if err != nil {
		t.Error(err)
		return
	}
	testFullFS(identifier, localPropFileName, t)
}

func TestFullFSInWD(t *testing.T) {
	identifier, localPropFileName, err := setupFullTestEnv()
	if err != nil {
		t.Error(err)
		return
	}
	baseDir, err = fslib.NewDirFromWD()
	if err != nil {
		t.Error(err)
		return
	}
	testFullFS(identifier, localPropFileName, t)
}

func testFullFS(identifier, localPropFileName string, t *testing.T) {
	baseDir.RemoveAll("*")
	server, err := baseDir.Create(identifier + ".jar")
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

	serverDir, err := CreateNewServerStructure(server.Path())
	if err != nil {
		t.Error(err)
		return
	}
	if serverDir.Path() != baseDir.Path()+identifier {
		t.Error("Server dir path is incorect")
		return
	}
	if !baseDir.Exists(fmt.Sprintf("/%[1]s/%[1]s.jar", identifier)) {
		t.Error("Server structure is not correct")
		return
	}
	instanceDir, err := CreateNewServerInstanceStructure(serverDir, server.Path(), typelib.RUNNING, "8080")
	if err != nil {
		t.Error(err)
		return
	}
	instanceDir.PrintTree()
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
