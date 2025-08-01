package main

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	ListenAddr           string `yaml:"listen_addr"`
	Socks5Addr           string `yaml:"socks5_addr"`
	UpdateFrequencyHours int    `yaml:"update_frequency_hours"`
}

var config Config

// setupConfig loads configuration with a layered approach:
// 1. Set default values.
// 2. Override with values from config.yaml if it exists.
// 3. Command-line flags will override these values later in main().
func setupConfig() {
	// 1. Set default values
	config = Config{
		ListenAddr:           ":8118",
		Socks5Addr:           "127.0.0.1:1080",
		UpdateFrequencyHours: 24,
	}

	// 2. Override with values from config.yaml
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Println("config.yaml not found, using default settings. This is not an error.")
		return // Continue with defaults
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		// If the file exists but is malformed, it's a fatal error.
		log.Fatalf("Failed to parse config.yaml: %v", err)
	}
	log.Println("Loaded configuration from config.yaml")
}
