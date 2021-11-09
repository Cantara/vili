package typelib

type ServerType int

const (
	UNKNOWN ServerType = iota
	RUNNING
	TESTING
)

func (t ServerType) String() string {
	return []string{"unknown", "running", "test"}[t]
}

func FromString(s string) ServerType {
	switch s {
	case RUNNING.String():
		return RUNNING
	case TESTING.String():
		return TESTING
	}
	return UNKNOWN
}
