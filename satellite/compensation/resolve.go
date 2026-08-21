// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"context"
	"net"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	badIPs = []net.IP{
		net.IPv4bcast,
		net.IPv4allsys,
		net.IPv4allrouter,
		net.IPv4zero,
		net.IPv6zero,
		net.IPv6unspecified,
		net.IPv6loopback,
		net.IPv6interfacelocalallnodes,
		net.IPv6linklocalallnodes,
		net.IPv6linklocalallrouters,
	}
)

// resolveNodesIPs resolves IPs for all invoices in parallel. Individual
// resolution failures do not fail the call; the returned int is the number of
// nodes for which no IPs could be resolved so the caller can decide whether
// that is acceptable.
func resolveNodesIPs(log *zap.Logger, concurrency int, invoices []Invoice) ([][]net.IP, int, error) {
	if concurrency == 0 {
		concurrency = 1
	}

	type input struct {
		n       int
		address string
		lastIP  string
	}

	type output struct {
		n   int
		ips []net.IP
		err error
	}

	inCh := make(chan input)
	outCh := make(chan output)

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Produce work for node IP resolution
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(inCh)
		for n, invoice := range invoices {
			in := input{
				n:       n,
				address: invoice.NodeAddress,
				lastIP:  invoice.NodeLastIP,
			}
			select {
			case inCh <- in:
			case <-ctx.Done():
				return
			}
		}
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range inCh {
				ips, err := resolveNodeIPs(ctx, in.address, in.lastIP)
				out := output{
					n:   in.n,
					ips: ips,
					err: err,
				}
				select {
				case outCh <- out:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	resolved := make([][]net.IP, len(invoices))
	var failures int
	for range invoices {
		out := <-outCh
		if out.err != nil {
			failures++
			log.Warn("failed to resolve IPs for node", zap.Stringer("node_id", invoices[out.n].NodeID), zap.Error(out.err))
		} else {
			resolved[out.n] = out.ips
		}
	}
	return resolved, failures, nil
}

func resolveNodeIPs(ctx context.Context, address, lastIP string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, address)
	nodeIPs := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		nodeIPs = append(nodeIPs, addr.IP)
	}
	if err != nil && lastIP == "" {
		return nil, errs.Wrap(err)
	}

	if lastIP != "" {
		parsed := net.ParseIP(lastIP)
		if parsed == nil {
			return nil, errs.New("last IP %q is not a valid IP address", lastIP)
		}
		nodeIPs = append(nodeIPs, parsed)
	}

	nodeIPs = cleanNodeIPs(nodeIPs...)
	if len(nodeIPs) == 0 {
		return nil, errs.New("no valid IP addresses")
	}
	return nodeIPs, nil
}

func cleanNodeIPs(nodeIPs ...net.IP) []net.IP {
	cleaned := make([]net.IP, 0, len(nodeIPs))
	for _, nodeIP := range nodeIPs {
		if isValidIP(nodeIP) {
			cleaned = append(cleaned, nodeIP)
		}
	}
	return cleaned
}

func isValidIP(ip net.IP) bool {
	if len(ip) == 0 {
		return false
	}
	for _, badIP := range badIPs {
		if badIP.Equal(ip) {
			return false
		}
	}
	return true
}
