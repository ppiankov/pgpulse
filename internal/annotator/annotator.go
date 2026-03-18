package annotator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Annotator pushes annotations to Grafana when anomalies are detected.
type Annotator struct {
	grafanaURL   string
	grafanaToken string
	dashboardUID string
	client       *http.Client

	mu   sync.Mutex
	prev map[string]float64 // metric name → previous value
}

// New creates an Annotator. Returns nil if Grafana URL is not configured.
func New(grafanaURL, grafanaToken, dashboardUID string) *Annotator {
	if grafanaURL == "" {
		return nil
	}
	a := &Annotator{
		grafanaURL:   grafanaURL,
		grafanaToken: grafanaToken,
		dashboardUID: dashboardUID,
		client:       &http.Client{Timeout: 10 * time.Second},
		prev:         make(map[string]float64),
	}
	log.Printf("anomaly annotations enabled: %s", grafanaURL)
	return a
}

// Check compares current value to previous and annotates if a spike is detected.
// A spike is defined as current > previous * threshold.
func (a *Annotator) Check(metric string, current float64, threshold float64, tags []string) {
	a.mu.Lock()
	prev, ok := a.prev[metric]
	a.prev[metric] = current
	a.mu.Unlock()

	if !ok || prev <= 0 {
		return
	}

	if threshold <= 0 {
		threshold = 2.0
	}

	if current > prev*threshold {
		text := fmt.Sprintf("%s spiked: %.0f → %.0f (%.1fx)", metric, prev, current, current/prev)
		a.push(text, tags)
	}
}

// CheckDelta annotates when an absolute increase exceeds a threshold.
func (a *Annotator) CheckDelta(metric string, current float64, minDelta float64, tags []string) {
	a.mu.Lock()
	prev, ok := a.prev[metric]
	a.prev[metric] = current
	a.mu.Unlock()

	if !ok {
		return
	}

	delta := current - prev
	if delta >= minDelta {
		text := fmt.Sprintf("%s jumped by %.0f (%.0f → %.0f)", metric, delta, prev, current)
		a.push(text, tags)
	}
}

func (a *Annotator) push(text string, tags []string) {
	allTags := append([]string{"pgpulse", "anomaly"}, tags...)

	body := map[string]interface{}{
		"text": text,
		"tags": allTags,
		"time": time.Now().UnixMilli(),
	}
	if a.dashboardUID != "" {
		body["dashboardUID"] = a.dashboardUID
	}

	payload, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/api/annotations", a.grafanaURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		log.Printf("annotation error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if a.grafanaToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.grafanaToken)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("annotation push error: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("annotation push failed: status %d", resp.StatusCode)
	}
}
