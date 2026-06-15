package kernel

type Health struct {
	ID        string
	State     ComponentState
	LastError string
}

type MsgBusStats struct {
	Published int64
	Dropped   int64
}
