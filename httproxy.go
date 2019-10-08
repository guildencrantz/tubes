package main

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
)

type httProxy struct {
	*http.Server
	dialer proxy.Dialer
}

func (p *httProxy) Finalize() {
	if p.Addr == "" {
		p.Addr = "127.0.0.1:3128" // 3128 is the default curl HTTP proxy port
	}

	p.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			p.handleTunneling(w, r)
		} else {
			p.handleHTTP(w, r)
		}
	})

	// Disable HTTP/2.
	p.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
}

func (p *httProxy) ListenAndServe() error {
	p.Finalize()

	l, err := net.Listen("tcp", p.Addr)
	if err != nil {
		return err
	}

	return p.Serve(l)
}

func (p *httProxy) handleTunneling(w http.ResponseWriter, r *http.Request) {
	dc, err := p.dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	cc, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	plumb(cc, dc)
}

func (p *httProxy) handleHTTP(w http.ResponseWriter, req *http.Request) {
	var t http.RoundTripper = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return p.dialer.Dial(network, addr)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
	}

	resp, err := t.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
