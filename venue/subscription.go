package venue

type Subscription interface {
	ID() string
	Close() error
	Done() <-chan struct{}
	Err() error
}
