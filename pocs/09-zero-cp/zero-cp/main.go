// zero-cp is the PoC control plane (arch §27, Sprint 05 P3): an OAuth2 token
// endpoint that grants a test token to registered sandbox SPIFFE IDs only.
package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type jwk struct {
	Kty, Use, Kid, Crv, X, Y, N, E string
}

type claims struct {
	Sub string `json:"sub"`
	Aud any    `json:"aud"`
	Exp int64  `json:"exp"`
}

func b64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func bigInt(s string) (*big.Int, error) {
	b, err := b64(s)
	return new(big.Int).SetBytes(b), err
}

func loadKeys(path string) (map[string]crypto.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var set struct{ Keys []jwk }
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, err
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if k.Use != "jwt-svid" {
			continue
		}
		switch k.Kty {
		case "RSA":
			n, err1 := bigInt(k.N)
			e, err2 := bigInt(k.E)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("bad RSA key %s", k.Kid)
			}
			keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
		case "EC":
			x, err1 := bigInt(k.X)
			y, err2 := bigInt(k.Y)
			if err1 != nil || err2 != nil || k.Crv != "P-256" {
				return nil, fmt.Errorf("bad EC key %s", k.Kid)
			}
			keys[k.Kid] = &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no usable keys in " + path)
	}
	return keys, nil
}

// verify checks signature, audience and expiry of a JWT-SVID and returns its SPIFFE ID.
func verify(token string, keys map[string]crypto.PublicKey, audience string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}
	var head struct{ Alg, Kid string }
	var c claims
	hb, err1 := b64(parts[0])
	cb, err2 := b64(parts[1])
	sig, err3 := b64(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || json.Unmarshal(hb, &head) != nil || json.Unmarshal(cb, &c) != nil {
		return "", errors.New("malformed JWT")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	switch key := keys[head.Kid].(type) {
	case *rsa.PublicKey:
		if head.Alg != "RS256" || rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
			return "", errors.New("bad signature")
		}
	case *ecdsa.PublicKey:
		if head.Alg != "ES256" || len(sig) != 64 ||
			!ecdsa.Verify(key, sum[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
			return "", errors.New("bad signature")
		}
	default:
		return "", errors.New("unknown signing key")
	}
	if time.Now().Unix() >= c.Exp {
		return c.Sub, errors.New("expired")
	}
	var auds []string
	switch a := c.Aud.(type) {
	case string:
		auds = []string{a}
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok {
				auds = append(auds, s)
			}
		}
	}
	if !slices.Contains(auds, audience) {
		return c.Sub, errors.New("wrong audience")
	}
	if !strings.HasPrefix(c.Sub, "spiffe://") {
		return c.Sub, errors.New("subject is not a SPIFFE ID")
	}
	return c.Sub, nil
}

func allowed(path, id string) bool {
	data, _ := os.ReadFile(path)
	return slices.Contains(strings.Fields(string(data)), id)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18090", "listen address")
	jwks := flag.String("jwks", "", "SPIRE JWT bundle (JWKS file)")
	audience := flag.String("audience", "", "required JWT-SVID audience")
	allow := flag.String("allow", "", "file with allowed SPIFFE IDs, re-read on every request")
	logPath := flag.String("log", "", "decision log (JSON lines)")
	svid := flag.Bool("svid", false, "grant the verified JWT-SVID itself as the token (PoC 13)")
	sessions := flag.String("sessions", "", "directory with one backend credential file per sandbox id (PoC 14)")
	tools := flag.String("tools", "", "directory with one tool list file per sandbox id; no file means no tool (PoC 15)")
	gatewayKey := flag.String("gateway-key", "", "file with the key the gateway sends to /authz/ (PoC 14)")
	flag.Parse()

	keys, err := loadKeys(*jwks)
	if err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	var mu sync.Mutex
	decide := func(id, decision, reason string) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(logFile).Encode(map[string]string{
			"time": time.Now().UTC().Format(time.RFC3339), "spiffe_id": id, "decision": decision, "reason": reason,
		})
	}
	deny := func(w http.ResponseWriter, id, reason string) {
		decide(id, "deny", reason)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
	}

	http.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		assertion := r.PostFormValue("client_assertion")
		if assertion == "" {
			deny(w, "", "missing client_assertion")
			return
		}
		id, err := verify(assertion, keys, *audience)
		if err != nil {
			deny(w, id, err.Error())
			return
		}
		if !allowed(*allow, id) {
			deny(w, id, "sandbox not registered")
			return
		}
		raw := make([]byte, 16)
		rand.Read(raw)
		token := "zerocp-test-" + hex.EncodeToString(raw)
		if *svid {
			token = assertion
		}
		decide(id, "allow", "registered sandbox")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 60})
	})
	// External authorization for a gateway (PoC 14). The gateway has verified the JWT-SVID (it does
	// not forward it); it proves itself with the gateway key and names the verified subject.
	// Answers with the backend credential of that session in a header only the gateway sees.
	http.HandleFunc("/authz/", func(w http.ResponseWriter, r *http.Request) {
		key, _ := os.ReadFile(*gatewayKey)
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(key) == 0 || subtle.ConstantTimeCompare([]byte(got), bytes.TrimSpace(key)) != 1 {
			decide("", "deny", "authz: caller is not the gateway")
			http.Error(w, "not the gateway", http.StatusForbidden)
			return
		}
		id := r.Header.Get("X-Zero-Subject")
		if !allowed(*allow, id) {
			decide(id, "deny", "authz: sandbox not registered")
			http.Error(w, "not registered", http.StatusForbidden)
			return
		}
		cred, err := os.ReadFile(filepath.Join(*sessions, filepath.Base(id)))
		if err != nil {
			decide(id, "deny", "authz: no backend credential for this session")
			http.Error(w, "no session", http.StatusForbidden)
			return
		}
		decide(id, "allow", "authz: backend credential of the session")
		if *tools != "" {
			list, _ := os.ReadFile(filepath.Join(*tools, filepath.Base(id)))
			w.Header().Set("X-Zero-Tools", strings.Join(strings.Fields(string(list)), ","))
		}
		w.Header().Set("X-Zero-Backend-Token", strings.TrimSpace(string(cred)))
	})
	// Test upstream: accepts only a granted token and never echoes it back.
	http.HandleFunc("GET /protected", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer zerocp-test-") {
			http.Error(w, "no granted token", http.StatusUnauthorized)
			return
		}
		fmt.Fprintln(w, "granted token accepted")
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}
