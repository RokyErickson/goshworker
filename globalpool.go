package goshworker

var PoolGlobal chan *proc
var isBigPoolInitialized bool

func NewPoolGlobal(size int, opts []string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if !isBigPoolInitialized {
		PoolGlobal = make(chan *proc, size)
		for i := 0; i < size; i++ {
			w := newProc(opts)
			PoolGlobal <- w
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
		close(PoolGlobal)
		size, _ := GetNumWorkersTotal()
		for i := 0; i < size; i++ {
			<-PoolGlobal
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

	return cap(PoolGlobal), nil
}

func GetNumWorkersAvail() (int, error) {
	if !isBigPoolInitialized {
		return 0, newError("Global worker pool was not initialized")
	}

	return len(PoolGlobal), nil
}
