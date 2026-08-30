package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type providerFunc func(context.Context, []Message) (string, error)

func (f providerFunc) Chat(ctx context.Context, m []Message) (string, error) { return f(ctx, m) }

func TestSessionsKeepSeparateHistory(t *testing.T) {
	p := providerFunc(func(_ context.Context, m []Message) (string, error) { return m[len(m)-1].Content, nil })
	s := New(p, Config{Timeout: time.Second, MaxContext: 2, MaxResponseChars: 100}, 1)
	a, _, _ := s.Ask(context.Background(), nil, "alpha")
	b, _, _ := s.Ask(context.Background(), nil, "beta")
	if strings.Contains(b[0].Content, "alpha") || a[0].Content == b[0].Content {
		t.Fatal("sessions shared conversation state")
	}
}
func TestProviderTimeout(t *testing.T) {
	p := providerFunc(func(ctx context.Context, _ []Message) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	s := New(p, Config{Timeout: 10 * time.Millisecond, MaxContext: 2, MaxResponseChars: 100}, 1)
	_, _, err := s.Ask(context.Background(), nil, "wait")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}
func TestConnectionError(t *testing.T) {
	want := errors.New("connection refused")
	s := New(providerFunc(func(context.Context, []Message) (string, error) { return "", want }), Config{Timeout: time.Second, MaxContext: 2, MaxResponseChars: 100}, 1)
	_, _, err := s.Ask(context.Background(), nil, "x")
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}
func TestResponseLimitAndMarkdown(t *testing.T) {
	s := New(providerFunc(func(context.Context, []Message) (string, error) { return "**123456789**", nil }), Config{Timeout: time.Second, MaxContext: 2, MaxResponseChars: 5}, 1)
	_, pages, err := s.Ask(context.Background(), nil, "x")
	if err != nil || strings.Join(pages, "") != "12345…" {
		t.Fatalf("%q %v", pages, err)
	}
}
