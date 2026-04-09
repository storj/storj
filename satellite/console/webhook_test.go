// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information

package console_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// mockWebhook simulates a webhook server.
type mockWebhook struct {
	mu       sync.Mutex
	payloads []map[string]string
	server   *httptest.Server
}

func newMockWebhook() *mockWebhook {
	wc := &mockWebhook{}
	wc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		wc.mu.Lock()
		wc.payloads = append(wc.payloads, payload)
		wc.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	return wc
}

func (wc *mockWebhook) findPayload(fn func(payload map[string]string) bool) map[string]string {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	for _, p := range wc.payloads {
		if fn(p) {
			return p
		}
	}
	return nil
}

func (wc *mockWebhook) close()      { wc.server.Close() }
func (wc *mockWebhook) url() string { return wc.server.URL }
