package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"staff-transport/internal/auth"
)

type fakeRepo struct {
	notifications map[string]*Notification
	tokens        map[string][]PushToken
	markedSent    string
	markedFailed  string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{notifications: map[string]*Notification{}, tokens: map[string][]PushToken{}}
}

func (f *fakeRepo) CreateNotification(_ context.Context, userID, typ string, payload map[string]any) (*Notification, error) {
	n := &Notification{ID: "n" + userID + typ, UserID: userID, Type: typ, Payload: payload, Status: StatusPending}
	f.notifications[n.ID] = n
	return n, nil
}

func (f *fakeRepo) GetNotification(_ context.Context, id string) (*Notification, error) {
	if n, ok := f.notifications[id]; ok {
		return n, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) MarkSent(_ context.Context, id string, _ time.Time) error {
	f.markedSent = id
	return nil
}

func (f *fakeRepo) MarkFailed(_ context.Context, id string) error {
	f.markedFailed = id
	return nil
}

func (f *fakeRepo) ListNotifications(_ context.Context, _ string, _, _ int) ([]Notification, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepo) UpsertToken(_ context.Context, userID, token, platform string) error {
	f.tokens[userID] = append(f.tokens[userID], PushToken{UserID: userID, Token: token, Platform: platform})
	return nil
}

func (f *fakeRepo) TokensForUser(_ context.Context, userID string) ([]PushToken, error) {
	return f.tokens[userID], nil
}

type fakeSender struct {
	calls  int
	tokens []string
	err    error
}

func (f *fakeSender) Send(_ context.Context, tokens []string, _ Message) error {
	f.calls++
	f.tokens = tokens
	return f.err
}

type fakeQueue struct {
	tasks int
}

func (f *fakeQueue) Enqueue(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks++
	return &asynq.TaskInfo{}, nil
}

func TestNotifyUsersCreatesAndEnqueues(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	svc := NewService(repo, &fakeSender{}, queue)

	err := svc.NotifyUsers(context.Background(), []string{"u1", "u1", "u2"}, TypeTripAssigned, map[string]any{"trip_id": "t1"})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(repo.notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(repo.notifications))
	}
	if queue.tasks != 2 {
		t.Fatalf("expected 2 enqueued tasks, got %d", queue.tasks)
	}
}

func TestHandleSendDeliversThenMarksSent(t *testing.T) {
	repo := newFakeRepo()
	repo.notifications["n1"] = &Notification{ID: "n1", UserID: "u1", Type: TypeTripAssigned, Status: StatusPending}
	repo.tokens["u1"] = []PushToken{{Token: "ExpoToken1"}, {Token: "ExpoToken2"}}
	sender := &fakeSender{}
	svc := NewService(repo, sender, nil)

	task, err := NewSendTask("n1")
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if err := svc.HandleSend(context.Background(), task); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if sender.calls != 1 || len(sender.tokens) != 2 {
		t.Fatalf("expected one send to 2 tokens, got %d/%d", sender.calls, len(sender.tokens))
	}
	if repo.markedSent != "n1" {
		t.Fatalf("expected n1 marked sent, got %q", repo.markedSent)
	}
}

func TestHandleSendWithoutTokensMarksFailed(t *testing.T) {
	repo := newFakeRepo()
	repo.notifications["n1"] = &Notification{ID: "n1", UserID: "u1", Status: StatusPending}
	sender := &fakeSender{}
	svc := NewService(repo, sender, nil)

	task, _ := NewSendTask("n1")
	if err := svc.HandleSend(context.Background(), task); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if sender.calls != 0 || repo.markedFailed != "n1" {
		t.Fatalf("expected failed with no send, got calls=%d failed=%q", sender.calls, repo.markedFailed)
	}
}

func TestHandleSendIsIdempotentWhenSent(t *testing.T) {
	repo := newFakeRepo()
	repo.notifications["n1"] = &Notification{ID: "n1", UserID: "u1", Status: StatusSent}
	sender := &fakeSender{}
	svc := NewService(repo, sender, nil)

	task, _ := NewSendTask("n1")
	if err := svc.HandleSend(context.Background(), task); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("expected no send for already-sent notification, got %d", sender.calls)
	}
}

func TestHandleSendPropagatesSenderError(t *testing.T) {
	repo := newFakeRepo()
	repo.notifications["n1"] = &Notification{ID: "n1", UserID: "u1", Status: StatusPending}
	repo.tokens["u1"] = []PushToken{{Token: "t"}}
	sender := &fakeSender{err: errors.New("boom")}
	svc := NewService(repo, sender, nil)

	task, _ := NewSendTask("n1")
	if err := svc.HandleSend(context.Background(), task); err == nil {
		t.Fatal("expected sender error to propagate for retry")
	}
	if repo.markedFailed != "n1" {
		t.Fatalf("expected failed mark, got %q", repo.markedFailed)
	}
}

func TestExpoSenderPostsMessages(t *testing.T) {
	var received []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("unexpected auth header %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	sender := &ExpoSender{client: server.Client(), endpoint: server.URL, accessToken: "secret"}
	err := sender.Send(context.Background(), []string{"ExpoTokenA"}, Message{Title: "Hi", Body: "There"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(received) != 1 || received[0]["to"] != "ExpoTokenA" {
		t.Fatalf("unexpected payload: %+v", received)
	}
}

type fakeAuth struct {
	claims map[string]*auth.Claims
}

func (f fakeAuth) Issue(context.Context, string, string) (auth.TokenPair, error) {
	return auth.TokenPair{}, nil
}
func (f fakeAuth) ConsumeRefresh(context.Context, string) (string, error) { return "", nil }
func (f fakeAuth) Revoke(context.Context, string) error                   { return nil }
func (f fakeAuth) ParseAccessToken(token string) (*auth.Claims, error) {
	if claims, ok := f.claims[token]; ok {
		return claims, nil
	}
	return nil, auth.ErrInvalidToken
}

func TestRegisterTokenRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo()
	svc := NewService(repo, &fakeSender{}, nil)
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}

	r := gin.New()
	NewHandler(svc, fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/push-tokens", strings.NewReader(`{"token":"ExpoToken","platform":"android"}`))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/me/push-tokens", strings.NewReader(`{"token":"ExpoToken","platform":"android"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer manager-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.tokens["m1"]) != 1 {
		t.Fatalf("expected token stored for m1, got %+v", repo.tokens)
	}
}
