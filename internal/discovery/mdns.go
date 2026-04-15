// Package discovery implements mDNS / Bonjour announce + browse so a
// client can find a RonTama server on the local network without
// knowing its IP.
//
// The service type is "_rontama._tcp"; instances are named after the
// host that's serving.
package discovery

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/hashicorp/mdns"
)

// ServiceType is the mDNS service type RonTama uses.
const ServiceType = "_rontama._tcp"

// Announce starts an mDNS service registration. Caller must Close() when done.
//
// host: the hostname to advertise (defaults to system hostname if empty).
// port: TCP port the server listens on.
// info: optional TXT records (e.g. ["rule=sichuan", "version=0.1.0"]).
func Announce(host string, port int, info []string) (io.Closer, error) {
	if host == "" {
		h, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("hostname: %w", err)
		}
		host = h
	}
	if !endsWithDot(host) {
		host = host + "."
	}

	addrs := publishableAddrs()
	svc, err := mdns.NewMDNSService(host, ServiceType, "", "", port, addrs, info)
	if err != nil {
		return nil, fmt.Errorf("mdns service: %w", err)
	}
	srv, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return nil, fmt.Errorf("mdns server: %w", err)
	}
	return announcer{srv}, nil
}

// announcer adapts mdns.Server (which has Shutdown, not Close) to io.Closer.
type announcer struct{ *mdns.Server }

func (a announcer) Close() error { return a.Shutdown() }

// Found is one discovered server entry.
type Found struct {
	Name string
	Host string
	Addr string // "ip:port"
	Info []string
}

// Browse returns the servers seen during the timeout window.
func Browse(ctx context.Context, timeout time.Duration) ([]Found, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	entries := make(chan *mdns.ServiceEntry, 16)
	defer close(entries)

	var found []Found
	doneCh := make(chan error, 1)
	go func() {
		params := mdns.DefaultParams(ServiceType)
		params.Entries = entries
		params.Timeout = timeout
		params.DisableIPv6 = true
		doneCh <- mdns.Query(params)
	}()

	deadline := time.After(timeout + 200*time.Millisecond)
	for {
		select {
		case e := <-entries:
			if e == nil {
				continue
			}
			ip := e.AddrV4
			if ip == nil {
				continue
			}
			found = append(found, Found{
				Name: e.Name,
				Host: e.Host,
				Addr: fmt.Sprintf("%s:%d", ip, e.Port),
				Info: e.InfoFields,
			})
		case <-doneCh:
			// Drain remaining entries.
			drainTimer := time.After(100 * time.Millisecond)
			for {
				select {
				case e := <-entries:
					if e == nil {
						continue
					}
					if e.AddrV4 == nil {
						continue
					}
					found = append(found, Found{
						Name: e.Name, Host: e.Host,
						Addr: fmt.Sprintf("%s:%d", e.AddrV4, e.Port),
						Info: e.InfoFields,
					})
				case <-drainTimer:
					return found, nil
				}
			}
		case <-deadline:
			return found, nil
		case <-ctx.Done():
			return found, ctx.Err()
		}
	}
}

func endsWithDot(s string) bool { return len(s) > 0 && s[len(s)-1] == '.' }

// publishableAddrs returns the IPv4 addresses suitable for mDNS announcement.
func publishableAddrs() []net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ipNet.IP.IsLoopback() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		out = append(out, ip4)
	}
	if len(out) == 0 {
		// Fall back to loopback so tests still work on a single host.
		return []net.IP{net.IPv4(127, 0, 0, 1)}
	}
	return out
}

