package proxy

import (
    "log"
    "net"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"

    "tailscale-proxy/internal/config"
)

type Router struct {
    cfg    config.Config
    rules  []rule
}

type rule struct {
    host       string
    prefix     string
    target     *url.URL
    rp         *httputil.ReverseProxy
}

func NewRouter(cfg config.Config) (http.Handler, error) {
    r := &Router{cfg: cfg}
    for _, rt := range cfg.Routes {
        rp := newReverseProxy(rt.Target, cfg.PreserveHost)
        r.rules = append(r.rules, rule{
            host:   strings.ToLower(rt.Host),
            prefix: rt.PathPrefix,
            target: rt.Target,
            rp:     rp,
        })
    }
    return r, nil
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // Choose rule: host exact match first, then longest path prefix
    host := strings.ToLower(req.Host)
    path := req.URL.Path
    if m := r.match(host, path); m != nil {
        // Optionally strip prefix
        if m.prefix != "" && strings.HasPrefix(path, m.prefix) {
            // Avoid stripping root
            if path != m.prefix {
                req.URL.Path = strings.TrimPrefix(path, m.prefix)
                if req.URL.Path == "" { req.URL.Path = "/" }
            }
        }
        m.rp.ServeHTTP(w, req)
        return
    }
    if r.cfg.DefaultBackend != nil {
        rp := newReverseProxy(r.cfg.DefaultBackend, r.cfg.PreserveHost)
        rp.ServeHTTP(w, req)
        return
    }
    http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func (r *Router) match(host, path string) *rule {
    var best *rule
    bestLen := -1
    for i := range r.rules {
        rl := &r.rules[i]
        if rl.host != "" && rl.host != host {
            continue
        }
        if rl.prefix == "" || strings.HasPrefix(path, rl.prefix) {
            plen := len(rl.prefix)
            if rl.host != "" { plen += 1000000 } // prefer exact host matches
            if plen > bestLen {
                bestLen = plen
                best = rl
            }
        }
    }
    return best
}

func newReverseProxy(target *url.URL, preserveHost bool) *httputil.ReverseProxy {
    director := func(req *http.Request) {
        req.URL.Scheme = target.Scheme
        req.URL.Host = target.Host
        // Join paths carefully
        req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
        // Set X-Forwarded headers
        if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
            appendHeader(req.Header, "X-Forwarded-For", ip)
        }
        if req.Header.Get("X-Forwarded-Proto") == "" {
            if req.TLS != nil {
                req.Header.Set("X-Forwarded-Proto", "https")
            } else {
                req.Header.Set("X-Forwarded-Proto", "http")
            }
        }
        if !preserveHost {
            req.Host = target.Host
        }
        // Remove hop-by-hop headers
        removeHopHeaders(req.Header)
    }
    rp := &httputil.ReverseProxy{Director: director}
    rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
        log.Printf("proxy error: %v", err)
        http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
    }
    return rp
}

func singleJoiningSlash(a, b string) string {
    aslash := strings.HasSuffix(a, "/")
    bslash := strings.HasPrefix(b, "/")
    switch {
    case aslash && bslash:
        return a + b[1:]
    case !aslash && !bslash:
        return a + "/" + b
    default:
        return a + b
    }
}

func appendHeader(h http.Header, k, v string) {
    if vv := h.Get(k); vv == "" {
        h.Set(k, v)
    } else {
        h.Set(k, vv+", "+v)
    }
}

func removeHopHeaders(h http.Header) {
    // Hop-by-hop headers per RFC 7230 §6.1
    hop := []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"}
    for _, k := range hop { h.Del(k) }
}

// contextCanceledErr returns a comparable error that matches context.Canceled
// (reserved for future nuanced error mapping)
