package config

import (
    "fmt"
    "net/url"
    "os"
    "sort"
    "strings"
)

type Config struct {
    ListenAddr     string
    PreserveHost   bool
    Routes         []Route
    DefaultBackend *url.URL
    AuthMode       string // off|shared_secret|google|both
    SharedSecret   string
    GoogleClientID string
    AllowedEmails  map[string]struct{}
    AllowedDomains map[string]struct{}
}

type Route struct {
    Host       string    // optional
    PathPrefix string    // optional, leading '/'
    Target     *url.URL  // required
}

func FromEnv() Config {
    cfg := Config{
        ListenAddr:   envDefault("LISTEN_ADDR", ":8080"),
        PreserveHost: strings.EqualFold(os.Getenv("PRESERVE_HOST"), "true"),
        AuthMode:     envDefault("AUTH_MODE", "off"),
        SharedSecret: os.Getenv("AUTH_SHARED_SECRET"),
        GoogleClientID: os.Getenv("GOOGLE_OIDC_CLIENT_ID"),
        AllowedEmails:  parseSet(os.Getenv("ALLOWED_EMAILS")),
        AllowedDomains: parseSet(os.Getenv("ALLOWED_DOMAINS")),
    }

    if def := os.Getenv("DEFAULT_BACKEND"); def != "" {
        if u, err := url.Parse(def); err == nil && u.Scheme != "" {
            cfg.DefaultBackend = u
        }
    }

    cfg.Routes = parseRoutes(os.Getenv("ROUTES"))
    // Sort routes: exact host first, longer PathPrefix first (for correct matching)
    sort.SliceStable(cfg.Routes, func(i, j int) bool {
        ri, rj := cfg.Routes[i], cfg.Routes[j]
        if ri.Host != rj.Host {
            return ri.Host != "" && rj.Host == ""
        }
        return len(ri.PathPrefix) > len(rj.PathPrefix)
    })

    return cfg
}

func envDefault(k, def string) string {
    v := os.Getenv(k)
    if v == "" {
        return def
    }
    return v
}

func parseSet(s string) map[string]struct{} {
    m := map[string]struct{}{}
    for _, p := range strings.Split(s, ",") {
        p = strings.TrimSpace(p)
        if p != "" {
            m[strings.ToLower(p)] = struct{}{}
        }
    }
    return m
}

// parseRoutes parses a comma-separated list of rules of the form
//   /api=http://host:port
//   host.example.com=http://host:port
//   host.example.com/prefix=http://host:port
func parseRoutes(s string) []Route {
    var routes []Route
    items := splitCSVRespectingURLs(s)
    for _, it := range items {
        it = strings.TrimSpace(it)
        if it == "" {
            continue
        }
        parts := strings.SplitN(it, "=", 2)
        if len(parts) != 2 {
            continue
        }
        left := strings.TrimSpace(parts[0])
        right := strings.TrimSpace(parts[1])
        u, err := url.Parse(right)
        if err != nil || u.Scheme == "" || u.Host == "" {
            continue
        }
        r := Route{Target: u}
        if strings.HasPrefix(left, "/") {
            r.PathPrefix = left
        } else if strings.Contains(left, "/") {
            // host + path
            i := strings.Index(left, "/")
            r.Host = strings.ToLower(left[:i])
            r.PathPrefix = left[i:]
        } else {
            // host only
            r.Host = strings.ToLower(left)
        }
        routes = append(routes, r)
    }
    return routes
}

// splitCSVRespectingURLs splits by commas not inside URL query/paths (best effort)
func splitCSVRespectingURLs(s string) []string {
    // Simple split: most URLs won't contain commas; if they do, quote the value in .env
    var out []string
    for _, p := range strings.Split(s, ",") {
        out = append(out, p)
    }
    return out
}

func (r Route) String() string {
    key := r.Host + r.PathPrefix
    if key == "" { key = "/" }
    return fmt.Sprintf("%s => %s", key, r.Target)
}

