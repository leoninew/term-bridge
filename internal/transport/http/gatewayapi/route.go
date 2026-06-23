package gatewayapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"

	"termbridge-go/internal/protocol/tunnel"
)

const requestTimeout = 5 * time.Second

type agentRoute struct {
	deviceId string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  map[tunnel.StreamId]chan tunnel.Frame
	terms    map[tunnel.StreamId]*terminalRelay
	mu       sync.Mutex
}

type terminalRelay struct {
	sessionId string
	browser   *websocket.Conn
	done      chan struct{}
}

func newAgentRoute(deviceId string, conn *websocket.Conn) *agentRoute {
	return &agentRoute{deviceId: deviceId, conn: conn, pending: map[tunnel.StreamId]chan tunnel.Frame{}, terms: map[tunnel.StreamId]*terminalRelay{}}
}

func (r *agentRoute) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	streamId := tunnel.StreamId(fmt.Sprintf("req-%d", time.Now().UnixNano()))
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	frame, err := tunnel.NewFrame(streamId, tunnel.FrameRequest, tunnel.RequestPayload{Method: method, Params: paramsData})
	if err != nil {
		return nil, err
	}
	ch := make(chan tunnel.Frame, 1)
	r.mu.Lock()
	r.pending[streamId] = ch
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, streamId)
		r.mu.Unlock()
	}()
	if err := r.writeFrame(ctx, frame); err != nil {
		return nil, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	case responseFrame := <-ch:
		response, err := tunnel.DecodePayload[tunnel.ResponsePayload](responseFrame)
		if err != nil {
			return nil, err
		}
		if !response.OK {
			return nil, errors.New(response.Error)
		}
		return response.Result, nil
	}
}

func (r *agentRoute) writeFrame(ctx context.Context, frame tunnel.Frame) error {
	data, err := tunnel.Encode(frame)
	if err != nil {
		return err
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.conn.Write(ctx, websocket.MessageText, data)
}

func (r *agentRoute) dispatch(frame tunnel.Frame) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch := r.pending[frame.StreamId]; ch != nil && (frame.Type == tunnel.FrameResponse || frame.Type == tunnel.FrameError) {
		ch <- frame
		return true
	}
	if term := r.terms[frame.StreamId]; term != nil {
		return term.dispatch(frame)
	}
	return false
}

func (r *agentRoute) addTerminal(streamId tunnel.StreamId, term *terminalRelay) {
	r.mu.Lock()
	r.terms[streamId] = term
	r.mu.Unlock()
}

func (r *agentRoute) removeTerminal(streamId tunnel.StreamId) {
	r.mu.Lock()
	delete(r.terms, streamId)
	r.mu.Unlock()
}

func (r *agentRoute) closeTerminals(reason string) {
	r.mu.Lock()
	terms := make([]*terminalRelay, 0, len(r.terms))
	for _, term := range r.terms {
		terms = append(terms, term)
	}
	r.terms = map[tunnel.StreamId]*terminalRelay{}
	r.mu.Unlock()
	for _, term := range terms {
		_ = term.browser.Write(context.Background(), websocket.MessageText, []byte(reason))
		_ = term.browser.Close(websocket.StatusGoingAway, reason)
		closeOnce(term.done)
	}
}

func (t *terminalRelay) dispatch(frame tunnel.Frame) bool {
	switch frame.Type {
	case tunnel.FrameTerminalOutput:
		payload, err := tunnel.DecodePayload[tunnel.TerminalDataPayload](frame)
		if err != nil {
			return true
		}
		_ = t.browser.Write(context.Background(), websocket.MessageBinary, payload.Data)
		return true
	case tunnel.FrameTerminalClosed, tunnel.FrameError, tunnel.FrameClose:
		_ = t.browser.Close(websocket.StatusNormalClosure, "terminal closed")
		closeOnce(t.done)
		return true
	default:
		return false
	}
}

func closeOnce(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
