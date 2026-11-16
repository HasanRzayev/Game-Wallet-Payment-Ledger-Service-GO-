package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type BenchmarkConfig struct {
	BaseURL     string
	Concurrency int
	TotalReqs   int
	PlayerID    string
	Currency    string
}

type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type WalletData struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
}

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Base URL of the game-wallet-ledger service")
	concurrency := flag.Int("concurrency", 20, "Number of concurrent worker goroutines")
	totalReqs := flag.Int("total", 200, "Total number of purchase requests to benchmark")
	flag.Parse()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	fmt.Printf("====================================================\n")
	fmt.Printf("   Game Wallet Ledger - Concurrency & Load Benchmark\n")
	fmt.Printf("====================================================\n")
	fmt.Printf("Target URL:      %s\n", *baseURL)
	fmt.Printf("Concurrency:     %d goroutines\n", *concurrency)
	fmt.Printf("Total Requests:  %d\n\n", *totalReqs)

	// Step 1: Check service health
	resp, err := client.Get(*baseURL + "/health")
	if err != nil {
		fmt.Printf("ERROR: Failed to connect to service at %s: %v\n", *baseURL, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("ERROR: /health returned status %d\n", resp.StatusCode)
		return
	}
	fmt.Println("[+] Service is healthy and responsive.")

	// Step 2: Create a dedicated benchmark wallet
	playerID := fmt.Sprintf("bench-player-%s", uuid.New().String()[:8])
	walletPayload, _ := json.Marshal(map[string]string{
		"player_id": playerID,
		"currency":  "GEMS",
	})
	walletResp, err := client.Post(*baseURL+"/api/v1/wallets", "application/json", bytes.NewBuffer(walletPayload))
	if err != nil {
		fmt.Printf("ERROR: Failed to create wallet: %v\n", err)
		return
	}
	defer walletResp.Body.Close()

	var walletResult APIResponse
	bodyBytes, _ := io.ReadAll(walletResp.Body)
	_ = json.Unmarshal(bodyBytes, &walletResult)
	var wallet WalletData
	_ = json.Unmarshal(walletResult.Data, &wallet)
	fmt.Printf("[+] Created benchmark wallet: ID=%s (Player=%s)\n", wallet.ID, playerID)

	// Step 3: Deposit ample balance for benchmark (e.g. 10,000,000 units)
	depositPayload, _ := json.Marshal(map[string]any{
		"amount":      10000000,
		"currency":    "GEMS",
		"description": "Benchmark initial funding",
	})
	depReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/wallets/%s/deposit", *baseURL, wallet.ID), bytes.NewBuffer(depositPayload))
	depReq.Header.Set("Content-Type", "application/json")
	depReq.Header.Set("Idempotency-Key", uuid.New().String())
	depResp, err := client.Do(depReq)
	if err != nil || depResp.StatusCode != http.StatusOK {
		fmt.Printf("ERROR: Failed to deposit funds for benchmark: %v\n", err)
		return
	}
	depResp.Body.Close()
	fmt.Printf("[+] Successfully deposited 10,000,000 GEMS into wallet %s\n\n", wallet.ID)

	// Step 4: Run concurrent purchase benchmark
	fmt.Printf("[*] Starting load test: sending %d purchase requests with %d concurrency...\n", *totalReqs, *concurrency)

	reqsPerWorker := *totalReqs / *concurrency
	var wg sync.WaitGroup
	var successCount int64
	var failureCount int64
	latencies := make([]time.Duration, 0, *totalReqs)
	var latenciesMu sync.Mutex

	benchmarkStart := time.Now()

	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < reqsPerWorker; i++ {
				idempKey := fmt.Sprintf("load-%s-%d-%d", wallet.ID, workerID, i)
				purchaseBody, _ := json.Marshal(map[string]any{
					"wallet_id":   wallet.ID,
					"amount":      10,
					"currency":    "GEMS",
					"item_sku":    "bench_chest_01",
					"description": fmt.Sprintf("Load test purchase w%d-i%d", workerID, i),
				})

				req, _ := http.NewRequest("POST", *baseURL+"/api/v1/payments/purchase", bytes.NewBuffer(purchaseBody))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", idempKey)

				t0 := time.Now()
				resp, err := client.Do(req)
				dur := time.Since(t0)

				latenciesMu.Lock()
				latencies = append(latencies, dur)
				latenciesMu.Unlock()

				if err == nil && resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
					resp.Body.Close()
				} else {
					atomic.AddInt64(&failureCount, 1)
					if resp != nil {
						resp.Body.Close()
					}
				}
			}
		}(w)
	}

	wg.Wait()
	totalDuration := time.Since(benchmarkStart)

	// Step 5: Analyze and display metrics
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}

	avgLatency := time.Duration(0)
	p50 := time.Duration(0)
	p95 := time.Duration(0)
	p99 := time.Duration(0)
	if len(latencies) > 0 {
		avgLatency = sum / time.Duration(len(latencies))
		p50 = latencies[len(latencies)*50/100]
		p95 = latencies[len(latencies)*95/100]
		p99 = latencies[len(latencies)*99/100]
	}

	rps := float64(*totalReqs) / totalDuration.Seconds()

	fmt.Printf("\n====================================================\n")
	fmt.Printf("                   BENCHMARK RESULTS                \n")
	fmt.Printf("====================================================\n")
	fmt.Printf("Total Requests:     %d\n", *totalReqs)
	fmt.Printf("Successful (200):   %d\n", successCount)
	fmt.Printf("Failed:             %d\n", failureCount)
	fmt.Printf("Total Elapsed Time: %v\n", totalDuration)
	fmt.Printf("Throughput (RPS):   %.2f req/sec\n", rps)
	fmt.Printf("Latency (Avg):      %v\n", avgLatency)
	fmt.Printf("Latency (P50):      %v\n", p50)
	fmt.Printf("Latency (P95):      %v\n", p95)
	fmt.Printf("Latency (P99):      %v\n", p99)
	fmt.Printf("====================================================\n")
}
