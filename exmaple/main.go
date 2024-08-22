package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	exserver "com.github.must11.exserver"
)

type countHandler struct {
	mu sync.Mutex // guards n
	n  int
}

func (h *countHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.n++
	fmt.Fprintf(w, "count is %d\n", h.n)
}

// 异步运行，5秒后主动关闭
func RunAndShutdown() {
	srv := exserver.NewExServer()
	srv.Listen(":8080", new(countHandler))
	srv.SyncRun(nil)
	time.Sleep(5 * time.Second)
	srv.Shutdown(context.Background())
	<-srv.ErrChan
}

// 异步运行，监听系统系统结束--
func RunAndSignalClose() {
	srv := exserver.NewExServer()

	srv.Listen(":8080", new(countHandler))
	srv.AddCloseSignal(os.Interrupt)
	srv.SyncRun(nil)
	<-srv.ErrChan
}

// 异步运行，等待运行结束--
func RunWaitClose() {
	srv := exserver.NewExServer()

	srv.Listen(":8080", new(countHandler))
	srv.AddCloseSignal(os.Interrupt)
	var wg sync.WaitGroup
	srv.SyncRun(&wg)
	go func() {
		time.Sleep(5 * time.Second)
		srv.Shutdown(context.Background())
	}()
	wg.Wait()
	fmt.Println("down")
}
func hellow() {
	fmt.Println("hello func")
}

// 运行结束时触发函数，用法同http.server
func RunWithCloseFunc() {
	srv := exserver.NewExServer()

	srv.AddCloseSignal(os.Interrupt)
	srv.RegisterOnShutdown(hellow)
	var wg sync.WaitGroup
	srv.Listen(":8080", new(countHandler)).SyncRun(&wg)
	go func() {
		time.Sleep(5 * time.Second)
		srv.Shutdown(context.Background())
	}()
	wg.Wait()
	fmt.Println("RunWithCloseFunc down")
}

func main() {
	RunAndShutdown()
	time.Sleep(5 * time.Second)
	RunAndSignalClose()
	time.Sleep(5 * time.Second)
	RunWaitClose()
	time.Sleep(5 * time.Second)
	RunWithCloseFunc()
}
