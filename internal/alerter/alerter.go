package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ppiankov/pgpulse/internal/config"
)

// AlertType identifies the kind of alert.
type AlertType string

const (
	AlertNodeDown       AlertType = "node_down"
	AlertConnSaturation AlertType = "conn_saturation"
	AlertLockChain      AlertType = "lock_chain"
	AlertRegression     AlertType = "regression"
	AlertSlowQuery      AlertType = "slow_query"
)

// Alert represents a single alert event.
type Alert struct {
	Type     AlertType
	Message  string
	Instance string
}

// Alerter sends notifications via configured channels.
type Alerter struct {
	telegramToken string
	telegramChat  string
	webhookURL    string
	cooldown      time.Duration
	client        *http.Client

	mu       sync.Mutex
	lastSent map[AlertType]time.Time
}

// New creates an Alerter from config. Returns nil if no notification targets are configured.
func New(cfg config.Config) *Alerter {
	hasTelegram := cfg.TelegramBotToken != "" && cfg.TelegramChatID != ""
	hasWebhook := cfg.AlertWebhookURL != ""

	if !hasTelegram && !hasWebhook {
		return nil
	}

	cooldown := cfg.AlertCooldown
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}

	a := &Alerter{
		telegramToken: cfg.TelegramBotToken,
		telegramChat:  cfg.TelegramChatID,
		webhookURL:    cfg.AlertWebhookURL,
		cooldown:      cooldown,
		client:        &http.Client{Timeout: 10 * time.Second},
		lastSent:      make(map[AlertType]time.Time),
	}

	if hasTelegram {
		log.Println("alerting enabled: Telegram")
	}
	if hasWebhook {
		log.Println("alerting enabled: webhook")
	}

	return a
}

// Fire sends an alert if the cooldown period has passed for this alert type.
func (a *Alerter) Fire(alert Alert) {
	a.mu.Lock()
	last, ok := a.lastSent[alert.Type]
	if ok && time.Since(last) < a.cooldown {
		a.mu.Unlock()
		return
	}
	a.lastSent[alert.Type] = time.Now()
	a.mu.Unlock()

	if a.telegramToken != "" && a.telegramChat != "" {
		a.sendTelegram(alert)
	}

	if a.webhookURL != "" {
		a.sendWebhook(alert)
	}
}

func (a *Alerter) sendTelegram(alert Alert) {
	text := fmt.Sprintf("<b>🔴 pgpulse alert: %s</b>\n\n%s\n\n<i>Instance: %s</i>",
		alert.Type, alert.Message, alert.Instance)

	body, _ := json.Marshal(map[string]string{
		"chat_id":    a.telegramChat,
		"text":       text,
		"parse_mode": "HTML",
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.telegramToken)
	resp, err := a.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("telegram alert error: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("telegram alert failed: status %d", resp.StatusCode)
	}
}

func (a *Alerter) sendWebhook(alert Alert) {
	payload, _ := json.Marshal(map[string]string{
		"text":     fmt.Sprintf("pgpulse alert [%s]: %s (instance: %s)", alert.Type, alert.Message, alert.Instance),
		"type":     string(alert.Type),
		"message":  alert.Message,
		"instance": alert.Instance,
	})

	resp, err := a.client.Post(a.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("webhook alert error: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("webhook alert failed: status %d", resp.StatusCode)
	}
}
