package model

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Stream sends a conversation as Chat does and hands each piece of the
// answer's text to said as it comes, so that a person reads it while it is
// written; it returns the whole turn, tool calls included, as Chat would.
func (o OpenAI) Stream(ctx context.Context, msgs []Message, tools []Tool, said func(string)) (Message, Usage, error) {
	body, err := o.body(msgs, tools, nil, true)
	if err != nil {
		return Message{}, Usage{}, err
	}
	resp, err := o.send(ctx, body)
	if err != nil {
		return Message{}, Usage{}, err
	}
	defer resp.Body.Close()
	var (
		text  strings.Builder
		calls []ToolCall
		use   *served
	)
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue // a comment the server keeps the line open with, or a blank
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *served `json:"usage"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return Message{}, Usage{}, fmt.Errorf("model: unreadable piece of an answer: %w", err)
		}
		if chunk.Error != nil {
			return Message{}, Usage{}, fmt.Errorf("model: %s broke off: %s", o.BaseURL, chunk.Error.Message)
		}
		if chunk.Usage != nil {
			use = chunk.Usage
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				text.WriteString(c.Delta.Content)
				if said != nil {
					said(c.Delta.Content)
				}
			}
			for _, d := range c.Delta.ToolCalls {
				for len(calls) <= d.Index {
					calls = append(calls, ToolCall{Type: "function"})
				}
				tc := &calls[d.Index]
				tc.ID += d.ID
				tc.Function.Name += d.Function.Name
				tc.Function.Arguments += d.Function.Arguments
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Message{}, Usage{}, fmt.Errorf("model: %s: %w", o.BaseURL, err)
	}
	calls = slices.DeleteFunc(calls, func(c ToolCall) bool { return c.Function.Name == "" })
	m := Message{Role: "assistant", Content: text.String(), ToolCalls: calls}
	if use == nil {
		if m.Content == "" && len(calls) == 0 {
			return m, Usage{Model: o.Name()}, errors.New("model: the answer was empty")
		}
		return m, Usage{Model: o.Name()}, nil
	}
	return m, o.usage(*use), nil
}
