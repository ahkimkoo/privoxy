package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var (
	directTransport http.RoundTripper
	proxyTransport  http.RoundTripper
	socks5Dialer    proxy.Dialer
	bufferPool      = sync.Pool{
		New: func() interface{} {
			return make([]byte, 32*1024)
		},
	}
)

func main() {
	// Setup configuration from defaults and config file
	setupConfig()

	// Define command-line flags, using config values as defaults
	flag.StringVar(&config.ListenAddr, "listen", config.ListenAddr, "Proxy listen address")
	flag.StringVar(&config.Socks5Addr, "socks5", config.Socks5Addr, "SOCKS5 proxy address")
	flag.IntVar(&config.UpdateFrequencyHours, "update-hours", config.UpdateFrequencyHours, "GFWList update frequency in hours")
	flag.Parse()

	// Initialize transports and dialer
	var err error
	socks5Dialer, err = proxy.SOCKS5("tcp", config.Socks5Addr, nil, proxy.Direct)
	if err != nil {
		log.Fatalf("Failed to create SOCKS5 dialer: %v", err)
	}

	directTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxyTransport = &http.Transport{
		Dial:                  socks5Dialer.Dial,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	loadCustomDomains()
	startGfwlistUpdater()

	log.Printf("Starting proxy server on %s", config.ListenAddr)
	log.Printf("Using SOCKS5 proxy: %s", config.Socks5Addr)
	server := &http.Server{
		Addr:    config.ListenAddr,
		Handler: http.HandlerFunc(handleRequest),
	}
	log.Fatal(server.ListenAndServe())
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Hostname()
	if host == "" {
		host = strings.Split(r.Host, ":")[0]
	}

	blocked := isBlocked(host)
	// Reduced logging for performance
	// log.Printf("Request for %s, host: %s, blocked: %v", r.URL.String(), host, blocked)

	if r.Method == http.MethodConnect {
		handleHTTPS(w, r, blocked)
	} else {
		handleHTTP(w, r, blocked)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request, blocked bool) {
	transport := directTransport
	if blocked {
		transport = proxyTransport
	}

	resp, err := transport.RoundTrip(r)
	if err != nil {
		log.Printf("Failed to get response: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func handleHTTPS(w http.ResponseWriter, r *http.Request, blocked bool) {
	var destConn net.Conn
	var err error

	if blocked {
		destConn, err = socks5Dialer.Dial("tcp", r.Host)
	} else {
		destConn, err = net.Dial("tcp", r.Host)
	}

	if err != nil {
		log.Printf("Failed to connect to destination %s: %v", r.Host, err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	defer destConn.Close()

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %v", err)
		return
	}

	tunnelData(clientConn, destConn)
}

// tunnelData uses io.CopyBuffer with a buffer pool to improve performance.
func tunnelData(client, target net.Conn) {
	defer client.Close()
	defer target.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		io.CopyBuffer(target, client, buf)
	}()

	go func() {
		defer wg.Done()
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		io.CopyBuffer(client, target, buf)
	}()

	wg.Wait()
}
