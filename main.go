package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Catalog struct {
	Provider    string            `json:"provider"`
	Service     string            `json:"service"`
	Endpoints   map[string]string `json:"endpoints"`
	Currency    string            `json:"currency"`
	PricingUnit string            `json:"pricingUnit"`
	Models      []Model           `json:"models"`
}

type Model struct {
	Name       string     `json:"name"`
	ID         string     `json:"id"`
	Pricing    Pricing    `json:"pricing"`
	Limits     Limits     `json:"limits"`
	Modalities Modalities `json:"modalities"`
	Cache      bool       `json:"cache"`
}

type Pricing struct {
	Input  []PriceRange `json:"input"`
	Output []PriceRange `json:"output"`
}

type PriceRange struct {
	Start                   int     `json:"start"`
	End                     *int    `json:"end"`
	TokenPriceUSDPerMillion float64 `json:"tokenPriceUsdPerMillion"`
}

type Limits struct {
	ContextWindowTokens int  `json:"contextWindowTokens"`
	MaxInputTokens      int  `json:"maxInputTokens"`
	MaxOutputTokens     int  `json:"maxOutputTokens"`
	MaxReasoningTokens  *int `json:"maxReasoningTokens"`
}

type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

var catalog = Catalog{
	Provider: "Huawei Cloud",
	Service:  "MaaS",
	Endpoints: map[string]string{
		"openaiCompatible":  "https://api-ap-southeast-1.modelarts-maas.com/openai/v1",
		"chatCompletions":   "https://api-ap-southeast-1.modelarts-maas.com/v2/chat/completions",
		"anthropicMessages": "https://api-ap-southeast-1.modelarts-maas.com/anthropic/v1/messages",
	},
	Currency:    "USD",
	PricingUnit: "1M tokens",
	Models: []Model{
		model("DeepSeek-V4-Pro", "deepseek-v4-pro", single(1.617), single(3.235), limits(1_000_000, 1_000_000, 128_000, ptr(96_000))),
		model("DeepSeek-V4-Flash", "deepseek-v4-flash", single(0.135), single(0.27), limits(1_000_000, 1_000_000, 128_000, ptr(96_000))),
		model("DeepSeek-V3.2", "deepseek-v3.2", single(0.27), single(0.404), limits(160_000, 128_000, 32_000, ptr(32_000))),
		model("DeepSeek-R1-0528", "deepseek-r1-0528", single(0.539), single(2.156), limits(128_000, 96_000, 32_000, ptr(32_000))),
		model("DeepSeek-V3", "DeepSeek-V3", single(0.27), single(1.078), limits(128_000, 128_000, 32_000, nil)),
		model("DeepSeek-V3.1-128K", "deepseek-v3.1-terminus", single(0.539), single(1.617), limits(128_000, 96_000, 32_000, ptr(32_000))),
		model("GLM-5.1", "glm-5.1", tiered(0.809, 1.078), tiered(3.235, 3.774), limits(198_000, 192_000, 128_000, ptr(96_000))),
		model("GLM-5", "glm-5", tiered(0.539, 0.809), tiered(2.426, 2.965), limits(198_000, 192_000, 64_000, ptr(64_000))),
	},
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local HTTP health endpoint")
	flag.Parse()

	if *healthcheck {
		if err := checkHealth(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("maas-model-catalog listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, catalog)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		log.Printf("write json response: %v", err)
	}
}

func checkHealth() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("invalid PORT %q: %w", port, err)
	}

	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %d", response.StatusCode)
	}
	return nil
}

func model(name, id string, input, output []PriceRange, limits Limits) Model {
	return Model{
		Name:    name,
		ID:      id,
		Pricing: Pricing{Input: input, Output: output},
		Limits:  limits,
		Modalities: Modalities{
			Input:  []string{"text"},
			Output: []string{"text"},
		},
		Cache: false,
	}
}

func limits(contextWindow, maxInput, maxOutput int, maxReasoning *int) Limits {
	return Limits{
		ContextWindowTokens: contextWindow,
		MaxInputTokens:      maxInput,
		MaxOutputTokens:     maxOutput,
		MaxReasoningTokens:  maxReasoning,
	}
}

func single(price float64) []PriceRange {
	return []PriceRange{{Start: 0, End: nil, TokenPriceUSDPerMillion: price}}
}

func tiered(lower, upper float64) []PriceRange {
	return []PriceRange{
		{Start: 0, End: ptr(31_999), TokenPriceUSDPerMillion: lower},
		{Start: 32_000, End: nil, TokenPriceUSDPerMillion: upper},
	}
}

func ptr(value int) *int {
	return &value
}
