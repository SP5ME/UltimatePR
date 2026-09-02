package service

import (
	"context"
	"errors"
	"io"
)

// Session is the common byte-stream contract used by services that need an
// outbound connection. Implementations remain responsible for their native
// transport and lifecycle, while callers only depend on this contract.
type Session interface {
	io.ReadWriteCloser
	Connect(context.Context, string) error
	Status() SessionStatus
}

type SessionStatus string

const (
	SessionIdle       SessionStatus = "idle"
	SessionConnecting SessionStatus = "connecting"
	SessionConnected  SessionStatus = "connected"
	SessionClosing    SessionStatus = "closing"
	SessionClosed     SessionStatus = "closed"
)

// SessionDialer creates a session using the Service Router.
type SessionDialer interface {
	DialSession(context.Context, SessionRequest) (Session, error)
}

type SessionRequest struct {
	Target       string
	Transport    string
	ViaNode      string
	FallbackToRF bool
	RFPort       string
}

var (
	ErrRouteNotFound      = errors.New("route not found")
	ErrConnectTimeout     = errors.New("connect timeout")
	ErrConnectionRejected = errors.New("connection rejected")
	ErrTransport          = errors.New("transport error")
	ErrSessionClosed      = errors.New("session closed")
)
