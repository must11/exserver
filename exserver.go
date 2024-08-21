package endless

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
)

type ExServer struct {
	srv         *http.Server
	running     atomic.Bool
	CloseSignal []os.Signal
	beforeRun   []func(es *ExServer)
	inShutdown  atomic.Bool // true when server is in shutdown
	addr        string
	handler     http.Handler
	closeChan   chan struct{}
	mu          sync.Mutex
}

func NewExServer() *ExServer {
	return &ExServer{
		beforeRun:   make([]func(es *ExServer), 0),
		CloseSignal: make([]os.Signal, 0),
	}
}

func (es *ExServer) Running() bool {
	return es.running.Load()
}

func (es *ExServer) SyncRun(errChannel chan error) {
	if es.Running() {
		return
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	if es.Running() {
		return
	}
	es.closeChan = make(chan struct{})
	es.srv = &http.Server{Addr: es.addr, Handler: es.handler}
	go func() {
		if err := es.srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("server listen err: %v", err)
			errChannel <- err
		} else {
			errChannel <- nil
		}
		close(es.closeChan)

	}()
	if len(es.CloseSignal) > 0 {
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, es.CloseSignal...)
			<-sigint
			es.Shutdown(context.Background())
		}()
	}
	es.running.Store(true)
	es.inShutdown.Store(false)
}

func (es *ExServer) Shutdown(ctx context.Context) error {
	if !es.Running() || es.srv == nil {
		log.Printf("server is not running  ")
		return nil
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	if err := es.srv.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		log.Printf("server shutdown err : %v", err)
		return err
	} else {
		log.Printf("server shutdown message : %v", err)
		<-es.closeChan
		es.running.Store(false)
		es.inShutdown.Store(true)
		return nil
	}

}

func (es *ExServer) Listen(addr string, handler http.Handler) {
	es.addr = addr
	es.handler = handler
}

func (es *ExServer) ReBoot(ctx context.Context, errChannel chan error) {
	if !es.running.Load() {
		es.SyncRun(errChannel)
	} else {
		es.Shutdown(ctx)
		if err := <-errChannel; err != nil {
			errChannel <- err
			return
		}
		es.SyncRun(errChannel)
	}
}

func NewErrChan() chan error {
	return make(chan error, 1)
}
