// zero-push is a PoC push service (#93): a git smart-HTTP proxy in front of a
// forge that forwards a push only if every ref update targets the allowed
// branch. It holds no credential; the Authorization header of the caller is
// passed through.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const zeroSHA = "0000000000000000000000000000000000000000"

type update struct{ Old, New, Ref string }

// readCommands reads the ref update commands of a receive-pack request up to
// the first flush packet.
func readCommands(r *bufio.Reader) ([]update, error) {
	var ups []update
	for {
		head := make([]byte, 4)
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		n, err := strconv.ParseUint(string(head), 16, 16)
		if err != nil {
			return nil, fmt.Errorf("bad pkt-line")
		}
		if n == 0 {
			return ups, nil
		}
		if n < 4 {
			return nil, fmt.Errorf("unsupported pkt-line")
		}
		line := make([]byte, n-4)
		if _, err := io.ReadFull(r, line); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		s, _, _ := strings.Cut(string(line), "\x00")
		f := strings.Fields(s)
		if len(f) != 3 || len(f[0]) != 40 || len(f[1]) != 40 {
			return nil, fmt.Errorf("unsupported command %q", s)
		}
		ups = append(ups, update{f[0], f[1], f[2]})
	}
}

func check(ups []update, ref string) error {
	for _, u := range ups {
		if u.Ref != ref {
			return fmt.Errorf("%s: only %s may be pushed", u.Ref, ref)
		}
		if u.New == zeroSHA {
			return fmt.Errorf("%s: delete not allowed", u.Ref)
		}
	}
	return nil
}

// deny answers in the receive-pack result format so git shows the reason.
func deny(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	line := "\x03zero-push: denied: " + msg + "\n"
	fmt.Fprintf(w, "%04x%s0000", len(line)+4, line)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18095", "listen address")
	upstream := flag.String("upstream", "https://gitlab.com", "forge base URL")
	repo := flag.String("repo", "", "repository path, e.g. /group/project.git")
	ref := flag.String("ref", "", "the only ref that may be pushed, e.g. refs/heads/zero/s1")
	session := flag.String("session", "", "session name for the log")
	logPath := flag.String("log", "", "decision log (JSON lines)")
	flag.Parse()
	if *repo == "" || *ref == "" || *logPath == "" {
		log.Fatal("need -repo, -ref and -log")
	}
	target, err := url.Parse(*upstream)
	if err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	var mu sync.Mutex
	record := func(u update, decision, reason string) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(logFile).Encode(map[string]string{
			"time": time.Now().UTC().Format(time.RFC3339), "session": *session,
			"ref": u.Ref, "old": u.Old, "new": u.New, "decision": decision, "reason": reason,
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == *repo+"/info/refs" && r.URL.Query().Get("service") == "git-receive-pack":
			proxy.ServeHTTP(w, r)
		case r.Method == http.MethodPost && r.URL.Path == *repo+"/git-receive-pack":
			body := io.Reader(r.Body)
			if r.Header.Get("Content-Encoding") == "gzip" {
				zr, err := gzip.NewReader(r.Body)
				if err != nil {
					http.Error(w, "bad gzip body", http.StatusBadRequest)
					return
				}
				body = zr
				r.Header.Del("Content-Encoding")
			}
			br := bufio.NewReader(body)
			var seen strings.Builder
			ups, err := readCommands(bufio.NewReader(io.TeeReader(br, &seen)))
			if err == nil {
				err = check(ups, *ref)
			}
			if err != nil {
				for _, u := range ups {
					record(u, "deny", err.Error())
				}
				if len(ups) == 0 {
					record(update{}, "deny", err.Error())
				}
				deny(w, err.Error())
				return
			}
			for _, u := range ups {
				record(u, "allow", "")
			}
			// The tee'd reader may have buffered past the commands: replay
			// everything read so far, then the rest.
			r.Body = io.NopCloser(io.MultiReader(strings.NewReader(seen.String()), br))
			r.ContentLength = -1
			r.Header.Del("Content-Length")
			proxy.ServeHTTP(w, r)
		default:
			http.Error(w, "zero-push: only pushes to "+*repo+" are served", http.StatusForbidden)
		}
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}
