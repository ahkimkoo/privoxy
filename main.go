package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func main() {
	// Setup configuration from defaults and config file
	setupConfig()

	// Define command-line flags, using config values as defaults
	flag.StringVar(&config.ListenAddr, "listen", config.ListenAddr, "Proxy listen address")
	flag.StringVar(&config.Socks5Addr, "socks5", config.Socks5Addr, "SOCKS5 proxy address")
	flag.IntVar(&config.UpdateFrequencyHours, "update-hours", config.UpdateFrequencyHours, "GFWList update frequency in hours")
	flag.Parse()

	loadCustomDomains()
	startGfwlistUpdater()

	log.Printf("Starting proxy server on %s", config.ListenAddr)
	log.Printf("Using SOCKS5 proxy: %s", config.Socks5Addr)
	server := &http.Server{
		Addr:         config.ListenAddr,
		Handler:      http.HandlerFunc(handleRequest),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Hostname()
	if host == "" {
		// For CONNECT requests, host is in r.Host
		host = strings.Split(r.Host, ":")[0]
	}

	blocked := isBlocked(host)
	log.Printf("Request for %s, host: %s, blocked: %v", r.URL.String(), host, blocked)

	if r.Method == http.MethodConnect {
		handleHTTPS(w, r, blocked)
	} else {
		handleHTTP(w, r, blocked)
	}
}

func handleHTTP(w http.ResponseWriter, r *http.Request, blocked bool) {
	// Create a new request to avoid modifying the original
	outReq := new(http.Request)
	*outReq = *r

	// Set up the transport
	transport := &http.Transport{}
	if blocked {
		dialer, err := proxy.SOCKS5("tcp", config.Socks5Addr, nil, proxy.Direct)
		if err != nil {
			log.Printf("Failed to create SOCKS5 dialer: %v", err)
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}
		transport.Dial = dialer.Dial
	}

	// Execute the request
	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		log.Printf("Failed to get response: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy body
	io.Copy(w, resp.Body)
}

func handleHTTPS(w http.ResponseWriter, r *http.Request, blocked bool) {
	var destConn net.Conn
	var err error

	if blocked {
		dialer, err := proxy.SOCKS5("tcp", config.Socks5Addr, nil, proxy.Direct)
		if err != nil {
			log.Printf("Failed to create SOCKS5 dialer: %v", err)
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}
		destConn, err = dialer.Dial("tcp", r.Host)
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
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Tunnel the data
	go io.Copy(destConn, clientConn)
	io.Copy(clientConn, destConn)
}
