package ai

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider isolates the RF service from a concrete model API.
type Provider interface {
	Chat(context.Context, []Message) (string, error)
}
