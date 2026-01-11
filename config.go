// ABOUTME: Configuration management for canvas-report.
// ABOUTME: Handles loading, saving, and interactive setup of Canvas credentials.

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseURL     string `yaml:"base_url"`
	AccessToken string `yaml:"access_token"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "canvas-report", "config.yaml"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func runSetup() (*Config, error) {
	fmt.Println("No configuration found. Let's set it up.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Canvas URL (e.g., https://yourschool.instructure.com): ")
	baseURL, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")

	fmt.Print("API Token (from Canvas > Settings > New Access Token): ")
	token, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)

	cfg := &Config{
		BaseURL:     baseURL,
		AccessToken: token,
	}

	fmt.Println()
	fmt.Print("Testing connection... ")

	client := NewCanvasClient(cfg.BaseURL, cfg.AccessToken)
	observees, err := client.Observees()
	if err != nil {
		fmt.Println("failed!")
		return nil, fmt.Errorf("could not connect to Canvas: %w", err)
	}

	if len(observees) == 0 {
		fmt.Println("connected, but no students found.")
		fmt.Println("Make sure your parent observer account is linked to your child's account.")
		return nil, fmt.Errorf("no observed students found")
	}

	names := make([]string, len(observees))
	for i, o := range observees {
		names[i] = o.Name
	}
	fmt.Printf("found %d student(s): %s\n", len(observees), strings.Join(names, ", "))

	if err := saveConfig(cfg); err != nil {
		return nil, fmt.Errorf("could not save config: %w", err)
	}

	path, err := configPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine config path: %w", err)
	}
	fmt.Printf("\nConfiguration saved to %s\n\n", path)

	return cfg, nil
}
