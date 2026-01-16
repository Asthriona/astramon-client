package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/mem"
)

type Metrics struct {
    Hostname  string  `json:"hostname"`
    IP        string  `json:"ip"`
    CPU       float64 `json:"cpu"`
    RAM       float64 `json:"ram"`
    Timestamp int64   `json:"timestamp"`
}

const (
    // DEVELOPMENT VALUE! REMEMBER TO UPDATE TO https://monitoring.asthriona.space/api/heartbeat IN PRODUCTION!
    apiURL = "https://monitoring.asthriona.space/api/heartbeat"
    
    // Send metrics every 60 seconds
    sendInterval = 60 * time.Second
    
    // HTTP timeout
    httpTimeout = 10 * time.Second
)

func main() {
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    
    hostname, err := os.Hostname()
    if err != nil {
        log.Fatalf("Failed to get hostname: %v", err)
    }
    
    log.Printf("Starting monitoring client for %s", hostname)
    log.Printf("Reporting to: %s", apiURL)
    log.Printf("Send interval: %v", sendInterval)
    
    // Create HTTP client with timeout
    client := &http.Client{
        Timeout: httpTimeout,
    }
    
    // Send metrics immediately on startup
    if err := sendMetrics(client, hostname); err != nil {
        log.Printf("Initial send failed: %v", err)
    }
    
    // Then send every minute
    ticker := time.NewTicker(sendInterval)
    defer ticker.Stop()
    
    for range ticker.C {
        if err := sendMetrics(client, hostname); err != nil {
            log.Printf("Error sending metrics: %v", err)
            // Continue running even if send fails
        }
    }
}

func getPublicIP(client *http.Client) (string, error) {
    resp, err := client.Get("https://api.ipify.org?format=json")
    if err != nil {
        return "", fmt.Errorf("failed to get public IP: %w", err)
    }
    defer resp.Body.Close()
    
    var result struct {
        IP string `json:"ip"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("failed to decode IP response: %w", err)
    }
    
    return result.IP, nil
}

func sendMetrics(client *http.Client, hostname string) error {
    // Get public IP
    ip, err := getPublicIP(client)
    if err != nil {
        return fmt.Errorf("failed to get IP: %w", err)
    }
    
    // Get CPU percentage (sample over 1 second)
    cpuPercent, err := cpu.Percent(time.Second, false)
    if err != nil {
        return fmt.Errorf("failed to get CPU metrics: %w", err)
    }
    
    // Get memory info
    memInfo, err := mem.VirtualMemory()
    if err != nil {
        return fmt.Errorf("failed to get RAM metrics: %w", err)
    }
    
    metrics := Metrics{
        Hostname:  hostname,
        IP:        ip,
        CPU:       cpuPercent[0],
        RAM:       memInfo.UsedPercent,
        Timestamp: time.Now().Unix(),
    }
    
    // Marshal to JSON
    jsonData, err := json.Marshal(metrics)
    if err != nil {
        return fmt.Errorf("failed to marshal JSON: %w", err)
    }
    
    // Send POST request
    resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()
    
    // Check response status
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("server returned status %d", resp.StatusCode)
    }
    
    log.Printf("✓ Sent metrics: IP=%s, CPU=%.1f%%, RAM=%.1f%%", ip, metrics.CPU, metrics.RAM)
    return nil
}