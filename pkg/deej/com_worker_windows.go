package deej

import (
	"errors"
	"runtime"
	"sync"

	ole "github.com/go-ole/go-ole"
)

const (
	comSFalse         = 0x00000001
	comRPCChangedMode = 0x80010106
)

var errCOMWorkerStopped = errors.New("COM worker stopped")

// comWorker serializes all COM calls on a single, locked OS thread. COM
// interface pointers must not be used from arbitrary goroutines: with STA the
// pointers are apartment-bound, and even with MTA the thread must be
// initialized before using them.
type comWorker struct {
	jobs     chan func()
	quit     chan struct{}
	done     chan struct{}
	initDone chan struct{}
	initErr  error

	stopOnce sync.Once
}

func newCOMWorker() (*comWorker, error) {
	worker := &comWorker{
		jobs:     make(chan func()),
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
		initDone: make(chan struct{}),
	}

	go worker.run()

	<-worker.initDone
	if worker.initErr != nil {
		return nil, worker.initErr
	}

	return worker, nil
}

func (w *comWorker) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)

	cleanup, err := initializeCOM()
	if err != nil {
		w.initErr = err
		close(w.initDone)
		return
	}
	defer cleanup()

	close(w.initDone)

	for {
		select {
		case <-w.quit:
			return
		case job := <-w.jobs:
			job()
		}
	}
}

// call runs fn on the COM thread and waits for it to finish. It must never be
// called from within a job running on the worker itself, or it will deadlock.
func (w *comWorker) call(fn func()) error {
	done := make(chan struct{})

	job := func() {
		defer close(done)
		fn()
	}

	select {
	case w.jobs <- job:
	case <-w.quit:
		return errCOMWorkerStopped
	case <-w.done:
		return errCOMWorkerStopped
	}

	select {
	case <-done:
		return nil
	case <-w.done:
		return errCOMWorkerStopped
	}
}

func (w *comWorker) stop() {
	w.stopOnce.Do(func() {
		close(w.quit)
		<-w.done
	})
}

func initializeCOM() (func(), error) {
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err == nil {
		return ole.CoUninitialize, nil
	}

	oleError, ok := err.(*ole.OleError)
	if !ok {
		return nil, err
	}

	switch uint32(oleError.Code()) {
	case comSFalse:
		// COM is already initialized on this thread; balance this successful call.
		return ole.CoUninitialize, nil
	case comRPCChangedMode:
		// The thread already uses another apartment model. Reuse it, but do not
		// uninitialize COM that this call did not initialize.
		return func() {}, nil
	}

	return nil, err
}
