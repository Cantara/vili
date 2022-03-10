package servlet

import "github.com/cantara/vili/fslib"

type Servlet interface {
	ReliabilityScore() float64
	IncrementBreaking()
	IncrementErrors()
	IncrementWarnings()
	IncrementRequests()
	ResetTestData()
	IsRunning() bool
	Kill()
	Dir() fslib.Dir
	Port() string
}
