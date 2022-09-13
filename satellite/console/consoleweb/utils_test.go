// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		desc string
		r    *http.Request
		ip   string
	}{
		{
			desc: "'Forwarded' single 'for'",
			r: &http.Request{
				RemoteAddr: "10.5.2.23",
				Header:     map[string][]string{"Forwarded": {"for=172.17.5.10"}},
			},
			ip: "10.5.2.23 (172.17.5.10)",
		},
		{
			desc: "'Forwarded' multiple 'for'",
			r: &http.Request{
				RemoteAddr: "192.168.5.2",
				Header: map[string][]string{
					"Forwarded": {"for=172.31.254.250,for=172.17.5.10"},
				},
			},
			ip: "192.168.5.2 (172.31.254.250)",
		},
		{
			desc: "'Forwarded' multiple 'for' with space after comma",
			r: &http.Request{
				RemoteAddr: "10.5.2.23",
				Header: map[string][]string{
					"Forwarded": {"for=192.168.5.250, for=172.17.5.10"},
				},
			},
			ip: "10.5.2.23 (192.168.5.250)",
		},
		{
			desc: "'Forwarded' multiple 'for' with other pairs",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						"by=storj;for=172.31.254.15,for=172.17.5.10;host=example.test;proto=https",
						"for=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (172.31.254.15)",
		},
		{
			desc: "'Forwarded' single capitalized 'For'",
			r: &http.Request{
				RemoteAddr: "10.5.2.23",
				Header:     map[string][]string{"Forwarded": {"For=172.17.5.10"}},
			},
			ip: "10.5.2.23 (172.17.5.10)",
		},
		{
			desc: "'Forwarded' multiple capitalized 'For'",
			r: &http.Request{
				RemoteAddr: "192.168.5.2",
				Header: map[string][]string{
					"Forwarded": {"For=172.31.254.250,For=172.17.5.10"},
				},
			},
			ip: "192.168.5.2 (172.31.254.250)",
		},
		{
			desc: "'Forwarded' multiple uppercase 'For' with space after comma",
			r: &http.Request{
				RemoteAddr: "10.5.2.23",
				Header: map[string][]string{
					"Forwarded": {"FOR=192.168.5.250, For=172.17.5.10"},
				},
			},
			ip: "10.5.2.23 (192.168.5.250)",
		},
		{
			desc: "'Forwarded' multiple capitalized 'For' with other pairs",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						"by=storj;For=172.31.254.15,for=172.17.5.10;host=example.test;proto=https",
						"For=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (172.31.254.15)",
		},
		{
			desc: "'Forwarded' 'for' IPv4 with port",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						`by=storj;for="172.31.254.15:9089",for=172.17.5.10;host=example.test;proto=https`,
						"for=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (172.31.254.15)",
		},
		{
			desc: "'Forwarded' 'for' IPv6",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						`by=storj;for="6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7",for=172.17.5.10;host=example.test;proto=https`,
						"for=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7)",
		},
		{
			desc: "'Forwarded' 'for' IPv6 with port",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						`by=storj;for="[6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7]:7896",for=172.17.5.10;host=example.test;proto=https`,
						"for=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7)",
		},
		{
			desc: "'Forwarded' with a extension whose field name has 'for' postfix",
			r: &http.Request{
				RemoteAddr: "172.20.20.20",
				Header: map[string][]string{
					"Forwarded": {
						"by=storj;xfor=172.31.254.15;for=172.17.5.10,for=172.17.5.12;host=example.test;proto=https",
						"For=172.28.15.15",
					},
				},
			},
			ip: "172.20.20.20 (172.17.5.10)",
		},
		{
			desc: "'X-Forwarded-For' single IP",
			r: &http.Request{
				RemoteAddr: "192.168.50.2",
				Header:     map[string][]string{"X-Forwarded-For": {"172.31.254.80"}},
			},
			ip: "192.168.50.2 (172.31.254.80)",
		},
		{
			desc: "'X-Forwarded-For' multiple IPs",
			r: &http.Request{
				RemoteAddr: "192.168.50.2",
				Header: map[string][]string{
					"X-Forwarded-For": {"172.28.254.80, 192.168.80.25"},
				},
			},
			ip: "192.168.50.2 (172.28.254.80)",
		},
		{
			desc: "'X-Real-Ip'",
			r: &http.Request{
				RemoteAddr: "192.168.50.2",
				Header:     map[string][]string{"X-Real-Ip": {"172.31.254.85"}},
			},
			ip: "192.168.50.2 (172.31.254.85)",
		},
		{
			desc: "multiple headers",
			r: &http.Request{
				RemoteAddr: "10.5.2.23",
				Header: map[string][]string{
					"X-Forwarded-For": {"172.28.254.80, 192.168.80.25"},
					"Forwarded":       {"for=192.168.5.250, for=172.17.5.10"},
					"X-Real-Ip":       {"172.31.254.85"},
				},
			},
			ip: "10.5.2.23 (192.168.5.250)",
		},
		{
			desc: "no headers",
			r: &http.Request{
				RemoteAddr: "192.168.50.60",
			},
			ip: "192.168.50.60",
		},
	}

	for _, tC := range testCases {
		tC := tC
		t.Run(tC.desc, func(t *testing.T) {
			ip := getClientIP(tC.r)

			assert.Equal(t, tC.ip, ip)
		})
	}
}

func TestStripPort(t *testing.T) {
	testCases := []struct {
		desc string
		addr string
		exp  string
	}{
		{
			desc: "hostname no port",
			addr: "storj.test",
			exp:  "storj.test",
		},
		{
			desc: "hostname port",
			addr: "storj.test:1234",
			exp:  "storj.test",
		},
		{
			desc: "hostname invalid",
			addr: "storj:test:123:",
			exp:  "storj:test:123:",
		},
		{
			desc: "IPv4 no port",
			addr: "192.168.1.78",
			exp:  "192.168.1.78",
		},
		{
			desc: "IPv4 port",
			addr: "192.168.7.69:7888",
			exp:  "192.168.7.69",
		},
		{
			desc: "IPv4 invalid",
			addr: "1985:5849.15.15:8080:",
			exp:  "1985:5849.15.15:8080:",
		},
		{
			desc: "IPv6 no port",
			addr: "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7",
			exp:  "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7",
		},
		{
			desc: "IPv6 port",
			addr: "[6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7]:9898",
			exp:  "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7",
		},
		{
			desc: "IPv6 invalid not closing bracket",
			addr: "[6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7:9898",
			exp:  "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b",
		},
		{
			desc: "IPv6 invalid port without brackets",
			addr: "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7:9898",
			exp:  "6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7:9898",
		},
		{
			desc: "IPv6 invalid brackets no port",
			addr: "[6934:9e20:e075:a5f6:c8d2:21d1:124d:94b7]",
			exp:  "6934:9e20:e075:a5f6:c8d2:21d1:124",
		},
		{
			desc: "empty address",
			addr: "",
			exp:  "",
		},
		{
			desc: "invalid address bracket",
			addr: "[",
			exp:  "[",
		},
		{
			desc: "invalid address bracket-colon",
			addr: "[:",
			exp:  "[:",
		},
		{
			desc: "invalid address brackets",
			addr: "[]",
			exp:  "[]",
		},
		{
			desc: "invalid address colon",
			addr: ":",
			exp:  "",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			host := stripPort(tC.addr)
			require.Equal(t, tC.exp, host)
		})
	}
}
