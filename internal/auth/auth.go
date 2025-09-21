package auth

import (
    "crypto"
    "crypto/rsa"
    "crypto/sha256"
    "encoding/base64"
    "encoding/binary"
    "encoding/json"
    "errors"
    "log"
    "math/big"
    "net/http"
    "strings"
    "sync"
    "time"

    "tailscale-proxy/internal/config"
)

const (
    ModeOff          = "off"
    ModeSharedSecret = "shared_secret"
    ModeGoogle       = "google"
    ModeBoth         = "both"
)

type Authenticator struct {
    Mode           string
    sharedSecret   string
    googleClientID string
    allowedEmails  map[string]struct{}
    allowedDomains map[string]struct{}

    jwksMu   sync.Mutex
    jwks     map[string]*rsa.PublicKey // kid -> key
    jwksExp  time.Time
}

func NewAuthenticator(cfg config.Config) (*Authenticator, error) {
    a := &Authenticator{
        Mode:           strings.ToLower(cfg.AuthMode),
        sharedSecret:   strings.TrimSpace(cfg.SharedSecret),
        googleClientID: strings.TrimSpace(cfg.GoogleClientID),
        allowedEmails:  cfg.AllowedEmails,
        allowedDomains: cfg.AllowedDomains,
    }
    if a.Mode == ModeOff {
        return a, nil
    }
    if (a.Mode == ModeSharedSecret || a.Mode == ModeBoth) && a.sharedSecret == "" {
        return nil, errors.New("AUTH_MODE requires AUTH_SHARED_SECRET")
    }
    if (a.Mode == ModeGoogle || a.Mode == ModeBoth) && a.googleClientID == "" {
        return nil, errors.New("AUTH_MODE requires GOOGLE_OIDC_CLIENT_ID")
    }
    return a, nil
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
    if a == nil || a.Mode == ModeOff {
        return next
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if a.authorized(r) {
            next.ServeHTTP(w, r)
            return
        }
        w.Header().Set("WWW-Authenticate", "Bearer")
        http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
    })
}

func (a *Authenticator) authorized(r *http.Request) bool {
    authz := r.Header.Get("Authorization")
    token := ""
    if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
        token = strings.TrimSpace(authz[7:])
    }
    switch a.Mode {
    case ModeSharedSecret:
        return token != "" && token == a.sharedSecret
    case ModeGoogle:
        if token == "" {
            // allow id_token in cookie as alternative
            if c, err := r.Cookie("id_token"); err == nil {
                token = c.Value
            }
        }
        return a.verifyGoogleToken(token)
    case ModeBoth:
        if token == a.sharedSecret && token != "" {
            return true
        }
        if token == "" {
            if c, err := r.Cookie("id_token"); err == nil {
                token = c.Value
            }
        }
        return a.verifyGoogleToken(token)
    default:
        return true
    }
}

// Minimal Google OIDC verification using JWKS and RS256.
// Accepts an ID token from Google and validates: sig, iss, aud, exp;
// optionally enforces allowed emails/domains.
func (a *Authenticator) verifyGoogleToken(idToken string) bool {
    if idToken == "" {
        return false
    }
    headerJSON, payloadJSON, sig, signedPart, err := splitAndDecodeJWT(idToken)
    if err != nil { return false }

    var hdr struct {
        Alg string `json:"alg"`
        Kid string `json:"kid"`
        Typ string `json:"typ"`
    }
    if err := json.Unmarshal(headerJSON, &hdr); err != nil { return false }
    if !strings.EqualFold(hdr.Alg, "RS256") || hdr.Kid == "" {
        return false
    }
    pub := a.fetchKey(hdr.Kid)
    if pub == nil {
        // refresh and try once more
        a.refreshJWKS(true)
        pub = a.fetchKey(hdr.Kid)
        if pub == nil { return false }
    }

    h := sha256.Sum256(signedPart)
    if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
        return false
    }

    var claims struct {
        Iss            string `json:"iss"`
        Aud            string `json:"aud"`
        Exp            int64  `json:"exp"`
        Iat            int64  `json:"iat"`
        Email          string `json:"email"`
        EmailVerified  bool   `json:"email_verified"`
        HD             string `json:"hd"`
    }
    if err := json.Unmarshal(payloadJSON, &claims); err != nil { return false }
    if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" { return false }
    if claims.Aud != a.googleClientID { return false }
    if time.Unix(claims.Exp, 0).Before(time.Now().Add(-1 * time.Minute)) { return false }
    // Optional email checks
    if len(a.allowedEmails) > 0 || len(a.allowedDomains) > 0 {
        if claims.Email == "" || !claims.EmailVerified { return false }
        email := strings.ToLower(claims.Email)
        if _, ok := a.allowedEmails[email]; ok { return true }
        if i := strings.LastIndexByte(email, '@'); i > 0 {
            dom := email[i+1:]
            if _, ok := a.allowedDomains[dom]; ok { return true }
        }
        return false
    }
    return true
}

func (a *Authenticator) fetchKey(kid string) *rsa.PublicKey {
    a.jwksMu.Lock()
    defer a.jwksMu.Unlock()
    if a.jwks == nil || time.Now().After(a.jwksExp) {
        a.refreshJWKS(false)
    }
    if a.jwks == nil { return nil }
    return a.jwks[kid]
}

func (a *Authenticator) refreshJWKS(force bool) {
    if !force && time.Now().Before(a.jwksExp) {
        return
    }
    // Google's JWKS endpoint (OIDC):
    // In production, consider discovering via OIDC config.
    const jwksURL = "https://www.googleapis.com/oauth2/v3/certs"
    req, _ := http.NewRequest("GET", jwksURL, nil)
    // Small timeout via default client not set here; rely on server timeouts.
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("jwks fetch error: %v", err)
        return
    }
    defer resp.Body.Close()
    var body struct { Keys []jwk `json:"keys"` }
    if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
        log.Printf("jwks parse error: %v", err)
        return
    }
    keys := map[string]*rsa.PublicKey{}
    for _, k := range body.Keys {
        if k.Kty != "RSA" || k.N == "" || k.E == "" || k.Kid == "" { continue }
        pub, err := jwkToRSA(k.N, k.E)
        if err != nil { continue }
        keys[k.Kid] = pub
    }
    if len(keys) > 0 {
        a.jwks = keys
        a.jwksExp = time.Now().Add(10 * time.Minute)
    }
}

type jwk struct {
    Kty string `json:"kty"`
    Kid string `json:"kid"`
    Use string `json:"use"`
    Alg string `json:"alg"`
    N   string `json:"n"`
    E   string `json:"e"`
    X5c []string `json:"x5c"`
}

func jwkToRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
    nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
    if err != nil { return nil, err }
    eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
    if err != nil { return nil, err }
    nn := new(big.Int).SetBytes(nBytes)
    var ee int
    // Usually 65537
    if len(eBytes) < 4 {
        tmp := make([]byte, 4)
        copy(tmp[4-len(eBytes):], eBytes)
        ee = int(binary.BigEndian.Uint32(tmp))
    } else {
        ee = int(binary.BigEndian.Uint32(eBytes[len(eBytes)-4:]))
    }
    return &rsa.PublicKey{N: nn, E: ee}, nil
}

// Helpers for JWT
func splitAndDecodeJWT(tok string) (header, payload []byte, sig []byte, signed []byte, err error) {
    parts := strings.Split(tok, ".")
    if len(parts) != 3 {
        return nil, nil, nil, nil, errors.New("invalid token segments")
    }
    h, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil { return nil, nil, nil, nil, err }
    p, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil { return nil, nil, nil, nil, err }
    s, err := base64.RawURLEncoding.DecodeString(parts[2])
    if err != nil { return nil, nil, nil, nil, err }
    signed = []byte(parts[0] + "." + parts[1])
    return h, p, s, signed, nil
}

// (no additional helpers)
