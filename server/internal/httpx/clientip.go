package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP returns the address used to key rate limits. It is the direct peer
// (RemoteAddr). The header named by trustHeader (for example X-Real-IP) is
// believed only when the peer is a loopback address, that is the local nginx;
// from anyone else it is client-controlled and ignored. The header value must
// parse as one IP address (netip.ParseAddr); when it holds a comma-separated
// list, the last entry is used because that one was appended by the trusted
// proxy. Anything unparsable falls back to the peer.
func ClientIP(r *http.Request, trustHeader string) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host // not an IP (unix socket, test double): use as is
	}
	peer = normalize(peer)
	if trustHeader != "" && peer.IsLoopback() {
		if vals := r.Header.Values(trustHeader); len(vals) > 0 {
			v := vals[len(vals)-1]
			if i := strings.LastIndexByte(v, ','); i >= 0 {
				v = v[i+1:]
			}
			if ip, err := netip.ParseAddr(strings.TrimSpace(v)); err == nil {
				return normalize(ip).String()
			}
		}
	}
	return peer.String()
}

// normalize unmaps IPv4-in-IPv6 and drops any zone so one host has one key.
func normalize(a netip.Addr) netip.Addr {
	return a.Unmap().WithZone("")
}
