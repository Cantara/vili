package server

import "time"

type Server interface {
	NewTesting(string) error
	Deploy() error
	RestartRunning()
	RestartTesting()
	GetRunningVersion() string
	GetTestingVersion() string
	GetPortRunning() string
	GetPortTesting() string
	AddBreaking()
	AddRequestRunning()
	AddRequestTesting()
	HasRunning() bool
	HasTesting() bool
	TestingDuration() time.Duration
	Messuring() bool
	ResetTest()
	IsRunningRunning() bool
	IsTestingRunning() bool
	CheckReliability(string)
	Kill()
}
