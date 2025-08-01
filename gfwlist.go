package main

import (
	"bufio"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	gfwlistURL        = "https://raw.githubusercontent.com/gfwlist/gfwlist/master/gfwlist.txt"
	gfwlistLocalFile  = "gfwlist.txt"
	customDomainsFile = "domain.txt"
)

var (
	gfwlist            = make(map[string]bool)
	customDomains      = make(map[string]bool)
	gfwlistMutex       = &sync.RWMutex{}
	customDomainsMutex = &sync.RWMutex{}
)

// loadCustomDomains reads the user-defined domain list.
func loadCustomDomains() {
	file, err := os.Open(customDomainsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("%s not found, skipping custom domains.", customDomainsFile)
			return
		}
		log.Printf("Failed to open %s: %v", customDomainsFile, err)
		return
	}
	defer file.Close()

	newCustomDomains := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		newCustomDomains[line] = true
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning %s: %v", customDomainsFile, err)
		return
	}

	customDomainsMutex.Lock()
	customDomains = newCustomDomains
	customDomainsMutex.Unlock()

	log.Printf("Loaded %d custom domains.", len(customDomains))
}

// loadGfwlistFromFile loads the rules from the local gfwlist.txt file.
func loadGfwlistFromFile() error {
	log.Println("Loading GFW list from local file...")
	body, err := ioutil.ReadFile(gfwlistLocalFile)
	if err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return err
	}

	newGfwlist := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(decoded)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "[") {
			continue
		}
		if strings.HasPrefix(line, "||") {
			line = strings.TrimPrefix(line, "||")
		}
		if strings.HasPrefix(line, ".") {
			line = strings.TrimPrefix(line, ".")
		}
		newGfwlist[line] = true
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	gfwlistMutex.Lock()
	gfwlist = newGfwlist
	gfwlistMutex.Unlock()

	log.Printf("GFW list loaded successfully. Total rules: %d", len(gfwlist))
	return nil
}

// updateGfwlist downloads the latest GFW list and saves it locally.
func updateGfwlist() {
	log.Println("Downloading new GFW list via SOCKS5 proxy...")

	// Create a new HTTP client that uses the SOCKS5 proxy
	proxyClient := &http.Client{
		Transport: &http.Transport{
			Dial: socks5Dialer.Dial,
		},
	}

	resp, err := proxyClient.Get(gfwlistURL)
	if err != nil {
		log.Printf("Failed to fetch GFW list: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read GFW list response: %v", err)
		return
	}

	err = ioutil.WriteFile(gfwlistLocalFile, body, 0644)
	if err != nil {
		log.Printf("Failed to write local GFW list file: %v", err)
		return
	}

	// After updating, load the new rules into memory
	if err := loadGfwlistFromFile(); err != nil {
		log.Printf("Failed to load GFW list from new file: %v", err)
	}
}

// isBlocked checks if a domain is in the GFW list or custom domain list.
func isBlocked(domain string) bool {
	customDomainsMutex.RLock()
	if customDomains[domain] {
		customDomainsMutex.RUnlock()
		return true
	}
	customDomainsMutex.RUnlock()

	gfwlistMutex.RLock()
	defer gfwlistMutex.RUnlock()

	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts)-1; i++ {
		d := strings.Join(parts[i:], ".")
		if gfwlist[d] {
			return true
		}
	}
	return false
}

// startGfwlistUpdater manages the GFW list, loading it on startup and
// starting a goroutine to handle periodic updates.
func startGfwlistUpdater() {
	// Initial check and load/update
	fileInfo, err := os.Stat(gfwlistLocalFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Local GFW list not found, downloading for the first time.")
			updateGfwlist()
		} else {
			log.Printf("Failed to stat local GFW list file: %v", err)
		}
	} else {
		// File exists, check modification time
		if time.Since(fileInfo.ModTime()).Hours() > float64(config.UpdateFrequencyHours) {
			log.Println("Local GFW list is outdated, updating.")
			updateGfwlist()
		} else {
			// File is recent, just load it
			if err := loadGfwlistFromFile(); err != nil {
				log.Printf("Failed to load existing GFW list: %v", err)
			}
		}
	}

	// Start periodic update goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(config.UpdateFrequencyHours) * time.Hour)
		for range ticker.C {
			updateGfwlist()
		}
	}()
}
