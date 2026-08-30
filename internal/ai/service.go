package ai

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrQueueFull = errors.New("AI queue is full; try again later")

type Config struct {
	Timeout                                   time.Duration
	MaxContext, MaxResponseChars              int
	SystemPrompt                              string
	WelcomeMessage, GoodbyeMessage, LocalCall string
	PageChars, QueueSize                      int
}
type Service struct {
	Provider      Provider
	Config        Config
	slots, queued chan struct{}
}

func New(provider Provider, cfg Config, concurrency int) *Service {
	if concurrency < 1 {
		concurrency = 1
	}
	if cfg.PageChars < 1 {
		cfg.PageChars = 700
	}
	if cfg.QueueSize < 1 {
		cfg.QueueSize = 8
	}
	return &Service{Provider: provider, Config: cfg, slots: make(chan struct{}, concurrency), queued: make(chan struct{}, cfg.QueueSize)}
}

func (s *Service) Ask(ctx context.Context, history []Message, prompt string) ([]Message, []string, error) {
	select {
	case s.queued <- struct{}{}:
		defer func() { <-s.queued }()
	default:
		return history, nil, ErrQueueFull
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return history, nil, ctx.Err()
	}
	messages := append([]Message(nil), history...)
	if len(messages) == 0 && strings.TrimSpace(s.Config.SystemPrompt) != "" {
		messages = append(messages, Message{Role: "system", Content: s.Config.SystemPrompt})
	}
	messages = append(messages, Message{Role: "user", Content: prompt})
	limit := s.Config.MaxContext * 2
	if limit > 0 && len(messages) > limit {
		messages = append([]Message(nil), messages[len(messages)-limit:]...)
	}
	callCtx, cancel := context.WithTimeout(ctx, s.Config.Timeout)
	defer cancel()
	answer, err := s.Provider.Chat(callCtx, messages)
	if err != nil {
		return history, nil, err
	}
	answer = Format(answer, s.Config.MaxResponseChars)
	messages = append(messages, Message{Role: "assistant", Content: answer})
	return messages, Paginate(answer, s.Config.PageChars), nil
}

func (s *Service) ServeSession(call, lang string, in *bufio.Scanner, w io.Writer) {
	if strings.TrimSpace(s.Config.WelcomeMessage) == "" {
		fmt.Fprintf(w, "\r\nAI service for %s. Enter a question, Q to return.\r\nAI> ", call)
	} else {
		fmt.Fprint(w, "\r\n", expandSessionMessage(s.Config.WelcomeMessage, s.Config.LocalCall, call), "\r\nAI> ")
	}
	var history []Message
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if strings.EqualFold(line, "Q") || strings.EqualFold(line, "QUIT") {
			if strings.TrimSpace(s.Config.GoodbyeMessage) != "" {
				fmt.Fprint(w, expandSessionMessage(s.Config.GoodbyeMessage, s.Config.LocalCall, call), "\r\n")
			}
			return
		}
		if line == "" {
			fmt.Fprint(w, "AI> ")
			continue
		}
		next, pages, err := s.Ask(context.Background(), history, line)
		if err != nil {
			fmt.Fprintf(w, "AI error: %v\r\nAI> ", err)
			continue
		}
		history = next
		for i, p := range pages {
			fmt.Fprintf(w, "\r\nAI:\r\n%s\r\n", p)
			if i < len(pages)-1 {
				fmt.Fprint(w, "[M] more / [Q] quit: ")
				if !in.Scan() || strings.EqualFold(strings.TrimSpace(in.Text()), "Q") {
					return
				}
			}
		}
		fmt.Fprint(w, "AI> ")
	}
}

func expandSessionMessage(message, local, remote string) string {
	return strings.NewReplacer("{CALL}", local, "{REMOTE}", remote).Replace(strings.TrimSpace(message))
}

func Format(v string, max int) string {
	v = strings.ReplaceAll(v, "```", "")
	v = strings.ReplaceAll(v, "**", "")
	v = strings.ReplaceAll(v, "__", "")
	v = strings.TrimSpace(strings.ReplaceAll(v, "\r", ""))
	if max > 0 && len([]rune(v)) > max {
		r := []rune(v)
		v = string(r[:max]) + "…"
	}
	lines := strings.Split(v, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\r\n")
}
func Paginate(v string, max int) []string {
	r := []rune(v)
	if len(r) == 0 {
		return []string{"(empty response)"}
	}
	var out []string
	for len(r) > 0 {
		n := max
		if n > len(r) {
			n = len(r)
		}
		out = append(out, string(r[:n]))
		r = r[n:]
	}
	return out
}
