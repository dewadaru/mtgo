package network

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const dnsResolverKeepTime = 10 * time.Minute

type dnsResolverCacheEntry struct {
	ips       []string
	createdAt time.Time
}

func (c dnsResolverCacheEntry) Ok() bool {
	return time.Since(c.createdAt) < dnsResolverKeepTime
}

type dnsResolver struct {
	client     *dns.Client
	server     string
	cache      map[string]dnsResolverCacheEntry
	cacheMutex sync.RWMutex
}

func (d *dnsResolver) LookupA(hostname string) []string {
	key := "\x00" + hostname

	d.cacheMutex.RLock()
	entry, ok := d.cache[key]
	d.cacheMutex.RUnlock()

	if ok && entry.Ok() {
		return entry.ips
	}

	var ips []string

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeA)
	msg.RecursionDesired = true

	if resp, _, err := d.client.Exchange(msg, d.server); err == nil && resp.Rcode == dns.RcodeSuccess {
		for _, ans := range resp.Answer {
			if a, ok := ans.(*dns.A); ok {
				ips = append(ips, a.A.String())
			}
		}

		d.cacheMutex.Lock()
		d.cache[key] = dnsResolverCacheEntry{
			ips:       ips,
			createdAt: time.Now(),
		}
		d.cacheMutex.Unlock()
	}

	return ips
}

func (d *dnsResolver) LookupAAAA(hostname string) []string {
	key := "\x01" + hostname

	d.cacheMutex.RLock()
	entry, ok := d.cache[key]
	d.cacheMutex.RUnlock()

	if ok && entry.Ok() {
		return entry.ips
	}

	var ips []string

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeAAAA)
	msg.RecursionDesired = true

	if resp, _, err := d.client.Exchange(msg, d.server); err == nil && resp.Rcode == dns.RcodeSuccess {
		for _, ans := range resp.Answer {
			if aaaa, ok := ans.(*dns.AAAA); ok {
				ips = append(ips, aaaa.AAAA.String())
			}
		}

		d.cacheMutex.Lock()
		d.cache[key] = dnsResolverCacheEntry{
			ips:       ips,
			createdAt: time.Now(),
		}
		d.cacheMutex.Unlock()
	}

	return ips
}

func newDNSResolver(hostname string, httpClient *http.Client) *dnsResolver {
	if net.ParseIP(hostname).To4() == nil {
		// the hostname is an IPv6 address
		hostname = fmt.Sprintf("[%s]", hostname)
	}

	// Use DoH (DNS-over-HTTPS) if httpClient is provided
	client := &dns.Client{
		Net:     "https",
		Timeout: 5 * time.Second,
	}

	// If httpClient has custom transport, use it
	if httpClient != nil && httpClient.Transport != nil {
		if transport, ok := httpClient.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil {
			client.TLSConfig = transport.TLSClientConfig
		}
	}

	// Format server URL for DoH
	server := fmt.Sprintf("https://%s/dns-query", hostname)

	return &dnsResolver{
		client: client,
		server: server,
		cache:  map[string]dnsResolverCacheEntry{},
	}
}