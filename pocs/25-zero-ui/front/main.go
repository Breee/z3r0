// Stand-in for zero-cp's UI listener in PoC 25: one loopback port, LiteLLM's UI below /litellm/.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	listen, target := os.Args[1], os.Args[2]
	u, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listen {
			http.Error(w, "wrong host", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/litellm" || strings.HasPrefix(r.URL.Path, "/litellm/") {
			proxy.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	log.Fatal(http.ListenAndServe(listen, mux))
}
