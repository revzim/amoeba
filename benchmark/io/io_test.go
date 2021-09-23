// +build benchmark

package io

import (
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/revzim/amoeba"
	"github.com/revzim/amoeba/benchmark/testdata"
	"github.com/revzim/amoeba/component"
	"github.com/revzim/amoeba/serialize/protobuf"
	"github.com/revzim/amoeba/session"
)

const (
	addr = "127.0.0.1:13250" // local address
	conc = 1000              // concurrent client count
)

//
type TestHandler struct {
	component.Base
	metrics int32
	group   *amoeba.Group
}

func (h *TestHandler) AfterInit() {
	ticker := time.NewTicker(time.Second)

	// metrics output ticker
	go func() {
		for range ticker.C {
			println("QPS", atomic.LoadInt32(&h.metrics))
			atomic.StoreInt32(&h.metrics, 0)
		}
	}()
}

func NewTestHandler() *TestHandler {
	return &TestHandler{
		group: amoeba.NewGroup("handler"),
	}
}

func (h *TestHandler) Ping(s *session.Session, data *testdata.Ping) error {
	atomic.AddInt32(&h.metrics, 1)
	return s.Push("pong", &testdata.Pong{Content: data.Content})
}

func server() {
	components := &component.Components{}
	components.Register(NewTestHandler())

	amoeba.Listen(addr,
		amoeba.WithDebugMode(),
		amoeba.WithSerializer(protobuf.NewSerializer()),
		amoeba.WithComponents(components),
	)
}

func client() {
	c := NewConnector()

	chReady := make(chan struct{})
	c.OnConnected(func() {
		chReady <- struct{}{}
	})

	if err := c.Start(addr); err != nil {
		panic(err)
	}

	c.On("pong", func(data interface{}) {})

	<-chReady
	for /*i := 0; i < 1; i++*/ {
		c.Notify("TestHandler.Ping", &testdata.Ping{})
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIO(t *testing.T) {
	go server()

	// wait server startup
	time.Sleep(1 * time.Second)
	for i := 0; i < conc; i++ {
		go client()
	}

	log.SetFlags(log.LstdFlags | log.Llongfile)

	sg := make(chan os.Signal)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)

	<-sg

	t.Log("exit")
}
