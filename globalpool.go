package goshworker

var GoshPoolGlobal chan *goshworker
var isBigPoolInitialized bool

func NewPoolGlobal(size int, opts []string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if !isBigPoolInitialized {
		GoshPoolGlobal = make(chan *goshworker, size)
		for i := 0; i < size; i++ {
			w := newGoshworker(opts)
			GoshPoolGlobal <- w
		}
		isBigPoolInitialized = true
		return nil
	}

	return newError("Global worker pool has been initialized")
}

func EndPoolGlobal() error {
	mutex.Lock()
	defer mutex.Unlock()
	if isBigPoolInitialized {
		close(GoshPoolGlobal)
		size, _ := GetNumWorkersTotal()
		for i := 0; i < size; i++ {
			<-GoshPoolGlobal
		}
		isBigPoolInitialized = false
		return nil
	}
	return newError("Global worker pool has been destroyed")
}

type poolErrorError struct {
	message string
}

func (err *poolErrorError) Error() string {
	return err.message
}

func newError(errMsg string) error {
	return &poolErrorError{
		message: errMsg,
	}
}

// GetNumWorkersTotal returns total number of workers created by the global worker pool
func GetNumWorkersTotal() (int, error) {
	if !isBigPoolInitialized {
		return 0, newError("Global worker pool was not initialized")
	}

	return cap(GoshPoolGlobal), nil
}

func GetNumWorkersAvail() (int, error) {
	if !isBigPoolInitialized {
		return 0, newError("Global worker pool was not initialized")
	}

	return len(GoshPoolGlobal), nil
}
