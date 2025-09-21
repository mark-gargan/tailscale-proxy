package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "tailscale-proxy/internal/auth"
    "tailscale-proxy/internal/config"
    "tailscale-proxy/internal/proxy"
)

func main() {
    // Load environment from .env if present (best-effort)
    loadDotEnv()

    cfg := config.FromEnv()

    // Build auth middleware based on config
    authenticator, err := auth.NewAuthenticator(cfg)
    if err != nil {
        log.Fatalf("auth init error: %v", err)
    }

    // Build proxy router
    router, err := proxy.NewRouter(cfg)
    if err != nil {
        log.Fatalf("router init error: %v", err)
    }

    handler := withMiddlewares(router, authenticator, cfg)

    srv := &http.Server{
        Addr:              cfg.ListenAddr,
        Handler:           handler,
        ReadHeaderTimeout: 10 * time.Second,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      60 * time.Second,
        IdleTimeout:       120 * time.Second,
    }

    go func() {
        log.Printf("listening on %s", cfg.ListenAddr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    // Graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    <-stop

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("graceful shutdown failed: %v", err)
    } else {
        log.Printf("server stopped")
    }
}

func withMiddlewares(next http.Handler, a *auth.Authenticator, cfg config.Config) http.Handler {
    h := next
    if a != nil && a.Mode != auth.ModeOff {
        h = a.Middleware(h)
    }
    h = requestLogger(h)
    h = recoverer(h)
    return securityHeaders(h)
}

func requestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ww := &statusWriter{ResponseWriter: w, status: 200}
        next.ServeHTTP(ww, r)
        dur := time.Since(start)
        log.Printf("%s %s host=%s status=%d dur=%s", r.Method, r.URL.Path, r.Host, ww.status, dur)
    })
}

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func recoverer(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                log.Printf("panic: %v", rec)
                http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}

func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Minimal hardening headers; adjust as needed for proxied apps
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "no-referrer")
        w.Header().Set("X-XSS-Protection", "0")
        next.ServeHTTP(w, r)
    })
}

func loadDotEnv() {
    // Minimal .env loader (no external deps)
    // Supports lines like KEY=VALUE, ignores comments and blanks
    data, err := os.ReadFile(".env")
    if err != nil {
        return
    }
    lines := strings.Split(string(data), "\n")
    for _, ln := range lines {
        ln = strings.TrimSpace(ln)
        if ln == "" || strings.HasPrefix(ln, "#") {
            continue
        }
        // split on first '='
        if i := strings.IndexByte(ln, '='); i > 0 {
            k := strings.TrimSpace(ln[:i])
            v := strings.TrimSpace(ln[i+1:])
            // remove optional surrounding quotes
            v = strings.Trim(v, "\"'")
            if os.Getenv(k) == "" {
                _ = os.Setenv(k, v)
            }
        }
    }
}

