package fslib

import (
	"io/fs"
	"time"
)

type File interface {
	Open() error
	Close() error
	Create() error
	NewFile(string) (File, error)
	NewDir(string) (File, error)
	Mkdir(fs.FileMode) error
	Remove() error
	RemoveAll() error
	Exists() bool
	Stat() (fs.FileInfo, error)
	Read([]byte) (int, error)
	ReadDir() ([]fs.DirEntry, error)
	Readdir() ([]File, error)
	Name() string
	Dir() string
	Path() string
	Size() int64
	Mode() fs.FileMode
	Type() fs.FileMode
	ModTime() time.Time
	IsDir() bool
	Parent() File
	Sys() interface{}
	Info() (fs.FileInfo, error)
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Symlink(string) error
	Readlink() (string, error)
}

type Dir interface {
	Open(string) (fs.File, error)
	Remove(string) error
	RemoveAll(string) error
	Create(string) (File, error)
	Mkdir(string, fs.FileMode) (Dir, error)
	Stat(string) (fs.FileInfo, error)
	Cd(string) (Dir, error)
	Path() string
	BaseDir() File
	Exists(string) bool
	ReadDir(string) ([]fs.DirEntry, error)
	Readdir(string) ([]File, error)
	ReadFile(string) ([]byte, error)
	Copy(File, string) error
	FindAndCopy(string, string) error
	Symlink(File, string) error
	Readlink(string) (string, error)
	File() File
	Find(string) (File, error)
	FindDir(string) (File, error)
	Size() int64
}
