// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"fmt"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"storj.io/common/memory"
)

// ContentLengthLimit describes 4KB limit.
const ContentLengthLimit = 4 * memory.KB

var _initAdditionalMimeTypes sync.Once

// initAdditionalMimeTypes initializes additional mime types,
// however we do it lazily to avoid needing to load OS mime types in tests.
func initAdditionalMimeTypes() {
	_initAdditionalMimeTypes.Do(func() {
		_ = mime.AddExtensionType(".ttf", "font/ttf")
		_ = mime.AddExtensionType(".txt", "text/plain")
	})
}

func typeByExtension(ext string) string {
	initAdditionalMimeTypes()
	return mime.TypeByExtension(ext)
}

// getClientIPRegExp is used by the function getClientIP.
var getClientIPRegExp = regexp.MustCompile(`(?i:(?:^|;)for=([^,; ]+))`)

// getClientIP gets the IP of the proxy (that's the value of the field
// r.RemoteAddr) and the client from the first exiting header in this order:
// 'Forwarded', 'X-Forwarded-For', or 'X-Real-Ip'.
// It returns a string of the format "{{proxy ip}} ({{client ip}})" or if there
// isn't any of those headers then it returns "{{client ip}}" where "client ip"
// is the value of the field r.RemoteAddr.
//
// The 'for' field of the 'Forwarded' may contain the IP with a port, as defined
// in the RFC 7239. When the header contains the IP with a port, the port is
// striped, so only the IP is returned.
//
// NOTE: it doesn't check that the IP value get from wherever source is a well
// formatted IP v4 nor v6; an invalid formatted IP will return an undefined
// result.
func getClientIP(r *http.Request) string {
	requestIPs := func(clientIP string) string {
		return fmt.Sprintf("%s (%s)", r.RemoteAddr, clientIP)
	}

	h := r.Header.Get("Forwarded")
	if h != "" {
		// Get the first value of the 'for' identifier present in the header because
		// its the one that contains the client IP.
		// see: https://datatracker.ietf.org/doc/html/rfc7230
		matches := getClientIPRegExp.FindStringSubmatch(h)
		if len(matches) > 1 {
			ip := strings.Trim(matches[1], `"`)
			ip = stripPort(ip)
			if ip[0] == '[' {
				ip = ip[1 : len(ip)-1]
			}

			return requestIPs(ip)
		}
	}

	h = r.Header.Get("X-Forwarded-For")
	if h != "" {
		// Get the first the value IP because it's the client IP.
		// Header sysntax: X-Forwarded-For: <client>, <proxy1>, <proxy2>
		// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
		ips := strings.SplitN(h, ",", 2)
		if len(ips) > 0 {
			return requestIPs(ips[0])
		}
	}

	h = r.Header.Get("X-Real-Ip")
	if h != "" {
		// Get the value of the header because its value is just the client IP.
		// This header is mostly sent by NGINX.
		// See https://www.nginx.com/resources/wiki/start/topics/examples/forwarded/
		return requestIPs(h)
	}

	return r.RemoteAddr
}

// stripPort strips the port from addr when it has it and return the host
// part. A host can be a hostname or an IP v4 or an IP v6.
//
// NOTE: this function expects a well-formatted address. When it's hostname or
// IP v4, the port at the end and separated with a colon, nor hostname or IP can
// have colons; when it's a IP v6 with port the IP part is enclosed with square
// brackets (.i.e []) and the port separated with a colon, otherwise the IP
// isn't enclosed by square brackets.
// An invalid addr produce an unspecified value.
func stripPort(addr string) string {
	// Ensure to strip the port out if r.RemoteAddr has it.
	// We don't use net.SplitHostPort because the function returns an error if the
	// address doesn't contain the port and the returned host is an empty string,
	// besides it doesn't return an error that can be distinguished from others
	// unless that the error message is compared, which is discouraging.
	if addr == "" {
		return ""
	}

	// It's an IP v6 with port.
	if addr[0] == '[' {
		idx := strings.LastIndex(addr, ":")
		if idx <= 1 {
			return addr
		}

		return addr[1 : idx-1]
	}

	// It's a IP v4 with port.
	if strings.Count(addr, ":") == 1 {
		idx := strings.LastIndex(addr, ":")
		if idx < 0 {
			return addr
		}

		return addr[0:idx]
	}

	// It's a IP v4 or v6 without port.
	return addr
}
