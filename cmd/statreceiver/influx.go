// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// InfluxDest is a MetricDest that sends data with the Influx TCP wire
// protocol
type InfluxDest struct {
	url   string
	token string

	mu      sync.Mutex
	buf     bytes.Buffer
	stopped bool
}

// NewInfluxDest creates a InfluxDest with stats URL url. Because
// this function is called in a Lua pipeline domain-specific language, the DSL
// wants a Influx destination to be flushing every few seconds, so this
// constructor will start that process. Use Close to stop it.
func NewInfluxDest(writeURL string) *InfluxDest {
	parsed, err := url.Parse(writeURL)
	if err != nil {
		panic(err)
	}
	token := parsed.Query().Get("authorization")
	parsed.Query().Del("authorization")

	rv := &InfluxDest{
		url:   parsed.String(),
		token: token,
	}
	go rv.flush()
	return rv
}

// Metric implements MetricDest
func (d *InfluxDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// TODO(jeff): actual parsing of the key is very tricky in the presence of influx's busted
	// escapes. if we could do that, we could more easily put the application tag in sorted order
	// but since it begins with a, we'll do the easy thing and insert it first.
	added := false
	for i, val := range key {
		if val != ' ' && val != ',' {
			continue
		} else if i == 0 {
			break
		} else if key[i-1] == '\\' {
			continue
		}

		var newKey []byte
		newKey = append(newKey, key[:i]...)
		newKey = append(newKey, ",application="...)
		newKey = appendTag(newKey, application)
		newKey = append(newKey, key[i:]...)
		key = newKey

		added = true
		break
	}
	if !added {
		// TODO(jeff): log that it was dropped? how?
		return nil
	}

	_, err := fmt.Fprintf(&d.buf, "%s=%v %d\n", key, val, ts.Truncate(time.Second).UnixNano())
	return err
}

// appendTag writes a tag key, value, or field key to the buffer.
func appendTag(buf []byte, tag string) []byte {
	if strings.IndexByte(tag, ',') == -1 &&
		strings.IndexByte(tag, '=') == -1 &&
		strings.IndexByte(tag, ' ') == -1 {

		return append(buf, tag...)
	}

	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' ||
			tag[i] == '=' ||
			tag[i] == ' ' {
			buf = append(buf, '\\')
		}
		buf = append(buf, tag[i])
	}

	return buf
}

// Close stops the flushing goroutine
func (d *InfluxDest) Close() error {
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	return nil
}

func (d *InfluxDest) flush() {
	for {
		time.Sleep(5 * time.Second)

		d.mu.Lock()
		if d.stopped {
			d.mu.Unlock()
			return
		}
		data := append([]byte{}, d.buf.Bytes()...)
		d.buf.Reset()
		d.mu.Unlock()

		log.Println("Sending", len(data), "bytes")

		req, err := http.NewRequest("POST", d.url, bytes.NewReader(data))
		if err == nil {
			if d.token != "" {
				req.Header.Set("Authorization", "Token "+d.token)
			}
			log.Printf("Req: %+v", req)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusNoContent {
					err = errs.New("invalid status code: %d", resp.StatusCode)
				}
			}
			log.Printf("Resp: %+v (%v)", resp, err)
		}
		if err != nil {
			log.Printf("failed flushing: %v", err)
		}
	}
}
