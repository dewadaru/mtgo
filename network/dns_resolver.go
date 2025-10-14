package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	doh "github.com/babolivier/go-doh-client"
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
	resolver   doh.Resolver
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

	if recs, _, err := d.resolver.LookupA(hostname); err == nil {
		for _, v := range recs {
			ips = append(ips, v.IP4)
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

	if recs, _, err := d.resolver.LookupAAAA(hostname); err == nil {
		for _, v := range recs {
			ips = append(ips, v.IP6)
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

	return &dnsResolver{
		resolver: doh.Resolver{
			Host:       hostname,
			Class:      doh.IN,
			HTTPClient: httpClient,
		},
		cache: map[string]dnsResolverCacheEntry{},
	}
}

type cachedResult struct {
	ipAddrs   []net.IPAddr
	timestamp time.Time
}

type Resolver struct {
	cache    sync.Map
	ttl      time.Duration
	resolver *net.Resolver
}

func (r *Resolver) LookupIP(host string) ([]net.IPAddr, error) {
	if val, ok := r.cache.Load(host); ok {
		res := val.(cachedResult)
		if time.Since(res.timestamp) < r.ttl {
			return res.ipAddrs, nil
		}
		r.cache.Delete(host)
	}
	addrs, err := r.resolver.LookupIPAddr(context.Background(), host)
	if err == nil {
		r.cache.Store(host, cachedResult{ipAddrs: addrs, timestamp: time.Now()})
	}
	return addrs, err
}
