package server

import "time"

type Server interface {
	NewTesting(string)
	Deploy()
	RestartRunning()
	RestartTesting()
	GetRunningVersion() string
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
