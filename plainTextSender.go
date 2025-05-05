package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type PlainTextReq struct {
	Lines []string
}

func (p *PlainTextReq) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	for _, line := range p.Lines {
		buf.WriteString(line)
	}
	return buf.Bytes(), nil
}

func NewPlainTextSender(opts LogSenderOpts) ISender {
	hdrs := opts.Headers
	opts.Headers = map[string]string{}
	for k, v := range hdrs {
		opts.Headers[k] = v
	}
	opts.Headers["Content-Type"] = "text/plain"

	var l *GenericSender
	l = &GenericSender{
		LogSenderOpts: opts,
		mtx:           sync.Mutex{},
		rnd:           rand.New(rand.NewSource(time.Now().UnixNano())),
		timeout:       time.Second * 10,
		path:          "/test-lines",
		generate: func() IRequest {
			lines := make([]string, 0, opts.LinesPS)
			for i := 0; i < opts.LinesPS; i++ {
				line := l.pickRandom(opts.Lines)
				lines = append(lines, line+"\n")
			}
			return &PlainTextReq{Lines: lines}
		},
	}

	return l
}
