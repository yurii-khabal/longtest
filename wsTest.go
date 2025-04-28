package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var latency = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:        "ws_lat",
	Help:        "test requests latency",
	ConstLabels: prometheus.Labels{"job": "longtest", "test": "ws-test"},
	Buckets:     []float64{0.05, 0.1, 0.3, 0.5, 1, 2, 10},
})

var logsReceived = promauto.NewCounter(prometheus.CounterOpts{
	Name:        "logs_received",
	ConstLabels: prometheus.Labels{"job": "longtest", "test": "ws-test"},
})

type WsTest struct {
	opts    LogSenderOpts
	writers []ISender
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWsTest(opts LogSenderOpts) ISender {
	return &WsTest{
		opts: opts,
	}
}

func (w *WsTest) Run() {
	w.ctx, w.cancel = context.WithCancel(context.Background())
	for i := 0; i < 100; i++ {
		go w.runReader()
	}
	time.Sleep(1 * time.Second)
	for i := 0; i < 1; i++ {
		w.runWriter()
	}
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()
}

func (w *WsTest) runWriter() {
	_opts := w.opts
	_opts.ID = fmt.Sprintf("writer-%d", len(w.writers))
	w.writers = append(w.writers, NewLogSender(_opts))
	w.writers[len(w.writers)-1].Run()
}

func (w *WsTest) runReader() {
	wsURL, err := url.Parse(w.opts.ReaderURL)
	if err != nil {
		panic(err)
	}
	wsURL.Scheme = strings.Replace(wsURL.Scheme, "http", "ws", 1)
	wsURL.Path = "/loki/api/v1/tail"
	values := url.Values{}
	values.Set("query", `{sender="logtest1", level=~"info|debug|error"}`)
	wsURL.RawQuery = values.Encode()

	customHeaders := http.Header{
		"X-Scope-OrgID":  {w.opts.Headers["X-Scope-OrgID"]},
		"X-Experimental": {"1"},
	}
	if wsURL.User.Username() != "" {
		p, _ := wsURL.User.Password()
		println(p)
		customHeaders.Set("X-API-Key", wsURL.User.Username())
		customHeaders.Set("X-API-Secret", p)
		wsURL.User = nil
	}
	println(wsURL.String())
	c, _, err := websocket.DefaultDialer.Dial(wsURL.String(), customHeaders)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	ch := make(chan string)
	go func() {
		defer close(ch)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			rcv := make(map[string][]LogStream)
			err = json.Unmarshal(message, &rcv)
			if err != nil {
				continue
			}
			for _, stream := range rcv["streams"] {
				for _, value := range stream.Values {
					ts, _ := strconv.ParseInt(value[0].(string), 10, 64)
					latency.Observe(float64(time.Now().UnixNano()-ts) / 1000000000)
					logsReceived.Inc()
				}
			}
		}
	}()

	for {
		select {
		case <-w.ctx.Done():
			return
		}
	}

}

func (w *WsTest) Stop() {
	w.cancel()
}
