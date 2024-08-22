package exserver

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
)

type ErrChan chan error

type ExServer struct {
	srv         *http.Server
	running     atomic.Bool
	closeSignal []os.Signal
	onShutdown  []func()
	inShutdown  atomic.Bool // true when server is in shutdown
	addr        string
	handler     http.Handler
	closeChan   chan struct{}
	mu          sync.Mutex
	ErrChan     ErrChan
}

func NewExServer() *ExServer {
	return &ExServer{
		onShutdown:  make([]func(), 0),
		closeSignal: make([]os.Signal, 0),
		ErrChan:     newErrChan(),
	}
}

func (es *ExServer) Running() bool {
	return es.running.Load()
}

// 异步启动http.srv
// wg 等待服务运行结束，在启动服务时add(1)，错误和关闭服务Done，如果不需要等待服务运行结束，设为nil
func (es *ExServer) SyncRun(wg *sync.WaitGroup) {
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
	for _, f := range es.onShutdown {
		es.srv.RegisterOnShutdown(f)
	}
	if wg != nil {
		wg.Add(1)
	}
	log.Printf("start listen at %s", es.addr)
	go func() {
		defer func() {
			if wg != nil {
				wg.Done()
			}
		}()
		if err := es.srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("server listen err: %v", err)
			es.ErrChan <- err
		} else {
			es.ErrChan <- nil
		}
		es.running.Store(false)
		es.inShutdown.Store(true)
		close(es.closeChan)

	}()
	if len(es.closeSignal) > 0 {
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, es.closeSignal...)
			<-sigint
			es.Shutdown(context.Background())
		}()
	}
	es.running.Store(true)
	es.inShutdown.Store(false)
}

func (es *ExServer) Shutdown(ctx context.Context) error {
	log.Print("serve runing", es.Running())
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
		return nil
	}

}
func (es *ExServer) AddCloseSignal(signal ...os.Signal) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.closeSignal = append(es.closeSignal, signal...)
}
func (es *ExServer) RemoveCloseSignal(signal os.Signal) {
	es.mu.Lock()
	defer es.mu.Unlock()
	newCloseSignal := make([]os.Signal, 0, len(es.closeSignal))
	for _, v := range es.closeSignal {
		if v != signal {
			newCloseSignal = append(newCloseSignal, v)
		}
	}
	es.closeSignal = newCloseSignal
}
func (es *ExServer) CleanCloseSignal() {
	es.closeSignal = es.closeSignal[:0]
}

func (es *ExServer) Listen(addr string, handler http.Handler) {
	es.addr = addr
	es.handler = handler
}

// 调用reboot,
// errChannel 必须跟之前保持一致
func (es *ExServer) ReBoot(ctx context.Context, wg *sync.WaitGroup) {
	log.Println("start reboot server")
	if !es.running.Load() {
		es.SyncRun(wg)
	} else {
		es.Shutdown(ctx)
		if err := <-es.ErrChan; err != nil {
			es.ErrChan <- err
			return
		}
		es.SyncRun(wg)
	}
}

// 同http.server.RegisterOnShutdown
func (es *ExServer) RegisterOnShutdown(f func()) {
	es.mu.Lock()
	es.onShutdown = append(es.onShutdown, f)
	es.mu.Unlock()
}

func newErrChan() ErrChan {
	return make(chan error, 1)
}
