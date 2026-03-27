package alerter

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ppiankov/pgpulse/internal/config"
)

func TestMaskDSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "standard DSN with user and password",
			dsn:      "postgres://admin:secret123@db.example.com:5432/mydb",
			expected: "postgres://a%2A%2A%2An:s%2A%2A%2A3@db.example.com:5432/mydb",
		},
		{
			name:     "DSN without password",
			dsn:      "postgres://readonly@db.example.com:5432/mydb",
			expected: "postgres://r%2A%2A%2Ay@db.example.com:5432/mydb",
		},
		{
			name:     "DSN without userinfo",
			dsn:      "postgres://db.example.com:5432/mydb",
			expected: "postgres://db.example.com:5432/mydb",
		},
		{
			name:     "short username and password",
			dsn:      "postgres://ab:cd@host:5432/db",
			expected: "postgres://%2A%2A%2A:%2A%2A%2A@host:5432/db",
		},
		{
			name:     "single char username and password",
			dsn:      "postgres://a:b@host:5432/db",
			expected: "postgres://%2A%2A%2A:%2A%2A%2A@host:5432/db",
		},
		{
			name:     "query parameters preserved",
			dsn:      "postgres://user:pass@host:5432/db?sslmode=require",
			expected: "postgres://u%2A%2A%2Ar:p%2A%2A%2As@host:5432/db?sslmode=require",
		},
		{
			name:     "unparseable DSN",
			dsn:      "not-a-url",
			expected: "<unparseable-dsn>",
		},
		{
			name:     "empty string",
			dsn:      "",
			expected: "<unparseable-dsn>",
		},
		{
			name:     "two char username",
			dsn:      "postgres://xy@host:5432/db",
			expected: "postgres://%2A%2A%2A@host:5432/db",
		},
		{
			name:     "three char username",
			dsn:      "postgres://abc@host:5432/db",
			expected: "postgres://a%2A%2A%2Ac@host:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MaskDSN(tt.dsn)
			if result != tt.expected {
				t.Errorf("MaskDSN(%q) = %q, want %q", tt.dsn, result, tt.expected)
			}
		})
	}
}

func TestHostFromDSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "standard DSN with port",
			dsn:      "postgres://user:pass@db.example.com:5432/mydb",
			expected: "db.example.com:5432",
		},
		{
			name:     "DSN without port",
			dsn:      "postgres://user:pass@db.example.com/mydb",
			expected: "db.example.com",
		},
		{
			name:     "unparseable DSN",
			dsn:      "not-a-url",
			expected: "unknown",
		},
		{
			name:     "empty string",
			dsn:      "",
			expected: "unknown",
		},
		{
			name:     "DSN with custom port",
			dsn:      "postgres://user:pass@localhost:6432/appdb",
			expected: "localhost:6432",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := HostFromDSN(tt.dsn)
			if result != tt.expected {
				t.Errorf("HostFromDSN(%q) = %q, want %q", tt.dsn, result, tt.expected)
			}
		})
	}
}

func TestMaskMiddle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "***",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "***",
		},
		{
			name:     "two characters",
			input:    "ab",
			expected: "***",
		},
		{
			name:     "three characters",
			input:    "abc",
			expected: "a***c",
		},
		{
			name:     "four characters",
			input:    "abcd",
			expected: "a***d",
		},
		{
			name:     "long string",
			input:    "secret123",
			expected: "s***3",
		},
		{
			name:     "admin",
			input:    "admin",
			expected: "a***n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := maskMiddle(tt.input)
			if result != tt.expected {
				t.Errorf("maskMiddle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cfg          config.Config
		wantNil      bool
		wantTelegram bool
		wantWebhook  bool
		cooldown     time.Duration
	}{
		{
			name:    "no targets configured",
			cfg:     config.Config{},
			wantNil: true,
		},
		{
			name: "telegram only",
			cfg: config.Config{
				TelegramBotToken: "test-token",
				TelegramChatID:   "test-chat",
			},
			wantNil:      false,
			wantTelegram: true,
		},
		{
			name: "webhook only",
			cfg: config.Config{
				AlertWebhookURL: "http://example.com/webhook",
			},
			wantNil:     false,
			wantWebhook: true,
		},
		{
			name: "both configured",
			cfg: config.Config{
				TelegramBotToken: "test-token",
				TelegramChatID:   "test-chat",
				AlertWebhookURL:  "http://example.com/webhook",
			},
			wantNil:      false,
			wantTelegram: true,
			wantWebhook:  true,
		},
		{
			name: "default cooldown when zero",
			cfg: config.Config{
				TelegramBotToken: "test-token",
				TelegramChatID:   "test-chat",
				AlertCooldown:    0,
			},
			wantNil:      false,
			cooldown:     5 * time.Minute,
			wantTelegram: true,
		},
		{
			name: "custom cooldown respected",
			cfg: config.Config{
				TelegramBotToken: "test-token",
				TelegramChatID:   "test-chat",
				AlertCooldown:    10 * time.Minute,
			},
			wantNil:      false,
			cooldown:     10 * time.Minute,
			wantTelegram: true,
		},
		{
			name: "negative cooldown becomes default",
			cfg: config.Config{
				TelegramBotToken: "test-token",
				TelegramChatID:   "test-chat",
				AlertCooldown:    -5 * time.Minute,
			},
			wantNil:      false,
			cooldown:     5 * time.Minute,
			wantTelegram: true,
		},
		{
			name: "telegram without chat ID returns nil",
			cfg: config.Config{
				TelegramBotToken: "test-token",
			},
			wantNil: true,
		},
		{
			name: "telegram without token returns nil",
			cfg: config.Config{
				TelegramChatID: "test-chat",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := New(tt.cfg)
			if tt.wantNil {
				if a != nil {
					t.Errorf("New() = %v, want nil", a)
				}
				return
			}
			if a == nil {
				t.Fatal("New() = nil, want non-nil")
			}
			if tt.cooldown > 0 && a.cooldown != tt.cooldown {
				t.Errorf("cooldown = %v, want %v", a.cooldown, tt.cooldown)
			}
			hasTelegram := a.telegramToken != "" && a.telegramChat != ""
			if tt.wantTelegram && !hasTelegram {
				t.Error("expected Telegram configured but it's not")
			}
			hasWebhook := a.webhookURL != ""
			if tt.wantWebhook && !hasWebhook {
				t.Error("expected webhook configured but it's not")
			}
		})
	}
}

func TestAlerterFireCooldown(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Alerter{
		webhookURL: server.URL,
		cooldown:   100 * time.Millisecond,
		client:     &http.Client{Timeout: 10 * time.Second},
		lastSent:   make(map[AlertType]time.Time),
	}

	alert1 := Alert{
		Type:     AlertNodeDown,
		Message:  "Node is down",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	alert2 := Alert{
		Type:     AlertConnSaturation,
		Message:  "Connection saturation",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	// First fire - should send
	a.Fire(alert1)
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request after first fire, got %d", len(requests))
	}
	mu.Unlock()

	// Second fire within cooldown - should be suppressed
	a.Fire(alert1)
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request after suppressed fire, got %d", len(requests))
	}
	mu.Unlock()

	// Different alert type - should NOT be suppressed
	a.Fire(alert2)
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests after different alert type, got %d", len(requests))
	}
	mu.Unlock()

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Same alert type after cooldown - should send again
	a.Fire(alert1)
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	if len(requests) != 3 {
		t.Fatalf("expected 3 requests after cooldown expiry, got %d", len(requests))
	}
	mu.Unlock()
}

func TestSendTelegram(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var receivedChatID string
	var receivedText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedChatID = payload["chat_id"]
		receivedText = payload["text"]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Alerter{
		telegramToken:   "test-token",
		telegramChat:    "test-chat-123",
		telegramBaseURL: server.URL,
		client:          &http.Client{Timeout: 10 * time.Second},
		lastSent:        make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertNodeDown,
		Message:  "Database node is unreachable",
		Instance: "postgres://a***n:s***t@db1.example.com:5432/mydb",
		Host:     "db1.example.com:5432",
	}

	a.sendTelegram(alert)

	if receivedChatID != "test-chat-123" {
		t.Errorf("chat_id = %q, want %q", receivedChatID, "test-chat-123")
	}

	if !strings.Contains(receivedText, "<b>🔴 pgpulse [db1.example.com:5432]: node_down</b>") {
		t.Errorf("text missing bold header, got: %q", receivedText)
	}

	if !strings.Contains(receivedText, "Database node is unreachable") {
		t.Errorf("text missing message, got: %q", receivedText)
	}

	if !strings.Contains(receivedText, alert.Instance) {
		t.Errorf("text missing DSN %q, got: %q", alert.Instance, receivedText)
	}
}

func TestSendWebhook(t *testing.T) {
	t.Parallel()

	var receivedPayload map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedPayload = payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := &Alerter{
		webhookURL: server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
		lastSent:   make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertLockChain,
		Message:  "Lock chain detected",
		Instance: "db-primary",
		Host:     "db1.example.com:5432",
	}

	a.sendWebhook(alert)

	if receivedPayload == nil {
		t.Fatal("no payload received")
	}

	if !strings.Contains(receivedPayload["text"], "pgpulse [db1.example.com:5432] alert [lock_chain]: Lock chain detected") {
		t.Errorf("text field incorrect, got: %q", receivedPayload["text"])
	}

	if receivedPayload["type"] != "lock_chain" {
		t.Errorf("type = %q, want %q", receivedPayload["type"], "lock_chain")
	}

	if receivedPayload["message"] != "Lock chain detected" {
		t.Errorf("message = %q, want %q", receivedPayload["message"], "Lock chain detected")
	}

	if receivedPayload["instance"] != "db-primary" {
		t.Errorf("instance = %q, want %q", receivedPayload["instance"], "db-primary")
	}

	if receivedPayload["host"] != "db1.example.com:5432" {
		t.Errorf("host = %q, want %q", receivedPayload["host"], "db1.example.com:5432")
	}
}

func TestSendTelegramServerError(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var logBuf bytes.Buffer
	origLog := log.Writer()
	log.SetOutput(&logBuf)
	defer func() {
		log.SetOutput(origLog)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := &Alerter{
		telegramToken:   "test-token",
		telegramChat:    "test-chat",
		telegramBaseURL: server.URL,
		client:          &http.Client{Timeout: 10 * time.Second},
		lastSent:        make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertNodeDown,
		Message:  "Test error",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	a.sendTelegram(alert)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "telegram alert failed") {
		t.Errorf("expected error log, got: %q", logOutput)
	}
}

func TestSendWebhookServerError(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var logBuf bytes.Buffer
	origLog := log.Writer()
	log.SetOutput(&logBuf)
	defer func() {
		log.SetOutput(origLog)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	a := &Alerter{
		webhookURL: server.URL,
		client:     &http.Client{Timeout: 10 * time.Second},
		lastSent:   make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertNodeDown,
		Message:  "Test error",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	a.sendWebhook(alert)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "webhook alert failed") {
		t.Errorf("expected error log, got: %q", logOutput)
	}
}

func TestSendTelegramNetworkError(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var logBuf bytes.Buffer
	origLog := log.Writer()
	log.SetOutput(&logBuf)
	defer func() {
		log.SetOutput(origLog)
	}()

	a := &Alerter{
		telegramToken: "test-token",
		telegramChat:  "test-chat",
		client:        &http.Client{Timeout: 100 * time.Millisecond},
		lastSent:      make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertNodeDown,
		Message:  "Test error",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	a.sendTelegram(alert)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "telegram alert error") {
		t.Errorf("expected error log, got: %q", logOutput)
	}
}

func TestSendWebhookNetworkError(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var logBuf bytes.Buffer
	origLog := log.Writer()
	log.SetOutput(&logBuf)
	defer func() {
		log.SetOutput(origLog)
	}()

	a := &Alerter{
		webhookURL: "http://invalid-host-that-does-not-exist.example.com/webhook",
		client:     &http.Client{Timeout: 100 * time.Millisecond},
		lastSent:   make(map[AlertType]time.Time),
	}

	alert := Alert{
		Type:     AlertNodeDown,
		Message:  "Test error",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	a.sendWebhook(alert)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "webhook alert error") {
		t.Errorf("expected error log, got: %q", logOutput)
	}
}

func TestFireBothChannels(t *testing.T) {
	// Cannot use t.Parallel() - uses global log state

	var telegramReceived bool
	var webhookReceived bool
	var mu sync.Mutex

	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		telegramReceived = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer telegramServer.Close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		webhookReceived = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := config.Config{
		TelegramBotToken: "test-token",
		TelegramChatID:   "test-chat",
		AlertWebhookURL:  webhookServer.URL,
	}
	a := New(cfg)
	if a == nil {
		t.Fatal("New() returned nil")
	}
	a.telegramBaseURL = telegramServer.URL

	alert := Alert{
		Type:     AlertSlowQuery,
		Message:  "Slow query detected",
		Instance: "db1",
		Host:     "db1.example.com",
	}

	a.Fire(alert)

	mu.Lock()
	defer mu.Unlock()

	if !telegramReceived {
		t.Error("Telegram alert not sent")
	}
	if !webhookReceived {
		t.Error("Webhook alert not sent")
	}
}
