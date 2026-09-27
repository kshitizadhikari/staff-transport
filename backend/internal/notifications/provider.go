package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Message is a provider-agnostic push message.
type Message struct {
	Title string
	Body  string
	Data  map[string]any
}

// Sender delivers a push message to one or more device tokens.
type Sender interface {
	Send(ctx context.Context, tokens []string, msg Message) error
}

const expoEndpoint = "https://exp.host/--/api/v2/push/send"

// ExpoSender delivers messages through the Expo push service. It also works
// with Expo's FCM-backed delivery, so the provider stays replaceable.
type ExpoSender struct {
	client      *http.Client
	endpoint    string
	accessToken string
}

// NewExpoSender returns an Expo push sender. accessToken may be empty.
func NewExpoSender(accessToken string) *ExpoSender {
	return &ExpoSender{
		client:      &http.Client{Timeout: 10 * time.Second},
		endpoint:    expoEndpoint,
		accessToken: accessToken,
	}
}

type expoMessage struct {
	To    string         `json:"to"`
	Title string         `json:"title,omitempty"`
	Body  string         `json:"body,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
	Sound string         `json:"sound,omitempty"`
}

func (e *ExpoSender) Send(ctx context.Context, tokens []string, msg Message) error {
	if len(tokens) == 0 {
		return nil
	}

	messages := make([]expoMessage, 0, len(tokens))
	for _, token := range tokens {
		messages = append(messages, expoMessage{
			To:    token,
			Title: msg.Title,
			Body:  msg.Body,
			Data:  msg.Data,
			Sound: "default",
		})
	}

	body, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if e.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+e.accessToken)
	}

	response, err := e.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("expo push request failed with status %d", response.StatusCode)
	}
	return nil
}
