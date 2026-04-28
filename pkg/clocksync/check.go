package clocksync

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	RemoteDate time.Time
	LocalNow   time.Time
	Skew       time.Duration
}

// Check compares local UTC time against HTTP Date header from a trusted endpoint.
func Check(checkURL string, timeout time.Duration) (*Result, error) {
	return CheckWithHeaders(checkURL, timeout, nil)
}

func CheckWithHeaders(checkURL string, timeout time.Duration, headers map[string]string) (*Result, error) {
	u := strings.TrimSpace(checkURL)
	if u == "" {
		return nil, fmt.Errorf("clock check url is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	req, err := http.NewRequest(http.MethodHead, u, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dateHeader := strings.TrimSpace(resp.Header.Get("Date"))
	if dateHeader == "" {
		return nil, fmt.Errorf("missing Date header from %s", u)
	}
	remoteDate, err := time.Parse(time.RFC1123, dateHeader)
	if err != nil {
		return nil, fmt.Errorf("parse Date header failed: %w", err)
	}
	localNow := time.Now().UTC()
	skew := localNow.Sub(remoteDate.UTC())
	if skew < 0 {
		skew = -skew
	}
	return &Result{
		RemoteDate: remoteDate.UTC(),
		LocalNow:   localNow,
		Skew:       skew,
	}, nil
}
