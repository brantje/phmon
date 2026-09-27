package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var errAgentWriterBackpressure = errors.New("agent writer queue full")

const agentWriterQueueSize = 32

type agentWriteRequest struct {
	value any
	done  chan error
}

type agentWriter struct {
	ctx    context.Context
	conn   *websocket.Conn
	cancel context.CancelFunc
	queue  chan agentWriteRequest
}

func newAgentWriter(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) *agentWriter {
	writer := &agentWriter{
		ctx:    ctx,
		conn:   conn,
		cancel: cancel,
		queue:  make(chan agentWriteRequest, agentWriterQueueSize),
	}
	go writer.run()
	return writer
}

func (w *agentWriter) Send(ctx context.Context, value any) error {
	request := agentWriteRequest{value: value, done: make(chan error, 1)}
	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case w.queue <- request:
	default:
		return errAgentWriterBackpressure
	}
	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case err := <-request.done:
		return err
	}
}

func (w *agentWriter) run() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case request := <-w.queue:
			writeCtx, cancel := context.WithTimeout(w.ctx, 2*time.Second)
			err := wsjson.Write(writeCtx, w.conn, request.value)
			cancel()
			request.done <- err
			if err != nil {
				w.cancel()
				return
			}
		}
	}
}
