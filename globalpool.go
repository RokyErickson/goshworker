package goshworker

import "github.com/RokyErickson/channels"

var GoshPoolGlobal channels.Channel
var isBigPoolInitialized bool

func NewPoolGlobal(size channels.BufferCap, opts []string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if !isBigPoolInitialized {
		GoshPoolGlobal = channels.NewNativeChannel(size)
		var workernum int = int(size)
		for i := 0; i < workernum; i++ {
			w := newGoshworker(opts)
			GoshPoolGlobal.In() <- w
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
		size, _ := GetNumWorkersTotal()
		var workernum int = int(size)
		for i := 0; i < workernum; i++ {
			<-GoshPoolGlobal.Out()
		}
		GoshPoolGlobal.Close()
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
func GetNumWorkersTotal() (channels.BufferCap, error) {
	if !isBigPoolInitialized {
		return 0, newError("Global worker pool was not initialized")
	}

	return GoshPoolGlobal.Cap(), nil
}

func GetNumWorkersAvail() (int, error) {
	if !isBigPoolInitialized {
		return 0, newError("Global worker pool was not initialized")
	}

	return GoshPoolGlobal.Len(), nil
}
