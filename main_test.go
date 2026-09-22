package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogIncludesEndpoints(t *testing.T) {
	expected := map[string]string{
		"openaiCompatible":  "https://api-ap-southeast-1.modelarts-maas.com/openai/v1",
		"chatCompletions":   "https://api-ap-southeast-1.modelarts-maas.com/v2/chat/completions",
		"anthropicMessages": "https://api-ap-southeast-1.modelarts-maas.com/anthropic/v1/messages",
	}

	if len(catalog.Endpoints) != len(expected) {
		t.Fatalf("expected %d endpoints, got %d", len(expected), len(catalog.Endpoints))
	}
	for key, value := range expected {
		if catalog.Endpoints[key] != value {
			t.Fatalf("endpoint %s = %q, want %q", key, catalog.Endpoints[key], value)
		}
	}
}

func TestCatalogIncludesRequestedModels(t *testing.T) {
	expected := []string{
		"DeepSeek-V4.1-Flash",
		"DeepSeek-V4-Flash",
		"DeepSeek-V4-Pro",
		"GLM-5.1",
		"GLM-5.2",
		"GLM-5.3",
	}

	if len(catalog.Models) != len(expected) {
		t.Fatalf("expected %d models, got %d", len(expected), len(catalog.Models))
	}
	for index, name := range expected {
		if catalog.Models[index].Name != name {
			t.Fatalf("model %d = %q, want %q", index, catalog.Models[index].Name, name)
		}
	}
}

func TestPriceRanges(t *testing.T) {
	for _, model := range catalog.Models {
		for direction, ranges := range map[string][]PriceRange{
			"input":  model.Pricing.Input,
			"output": model.Pricing.Output,
		} {
			if len(ranges) == 0 {
				t.Fatalf("%s %s pricing is empty", model.Name, direction)
			}
			for _, priceRange := range ranges {
				if priceRange.Start < 0 {
					t.Fatalf("%s %s range has negative start", model.Name, direction)
				}
				if priceRange.End != nil && *priceRange.End < priceRange.Start {
					t.Fatalf("%s %s range ends before it starts", model.Name, direction)
				}
				if priceRange.End == nil {
					t.Fatalf("%s %s range end is nil; pricing ranges must use a numeric end", model.Name, direction)
				}
				if priceRange.TokenPriceUSDPerMillion <= 0 {
					t.Fatalf("%s %s range has non-positive token price", model.Name, direction)
				}
			}
		}
	}
}

func TestTieredGLMPricing(t *testing.T) {
	glm51 := findModel("glm-5.1")

	assertRanges(t, glm51.Pricing.Input, []PriceRange{
		{Start: 0, End: ptr(31_999), TokenPriceUSDPerMillion: 0.809},
		{Start: 32_000, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: 1.078},
	})
	assertRanges(t, glm51.Pricing.Output, []PriceRange{
		{Start: 0, End: ptr(31_999), TokenPriceUSDPerMillion: 3.235},
		{Start: 32_000, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: 3.774},
	})

}

func TestGLM52Details(t *testing.T) {
	glm52 := findModel("glm-5.2")

	if glm52.Name != "GLM-5.2" {
		t.Fatalf("name = %q, want %q", glm52.Name, "GLM-5.2")
	}
	assertRanges(t, glm52.Pricing.Input, []PriceRange{
		{Start: 0, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: 1.40},
	})
	assertRanges(t, glm52.Pricing.Output, []PriceRange{
		{Start: 0, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: 4.40},
	})
	assertLimits(t, glm52.Limits, Limits{
		ContextWindowTokens: 1_000_000,
		MaxInputTokens:      1_000_000,
		MaxOutputTokens:     128_000,
		MaxReasoningTokens:  ptr(64_000),
	})
	assertStringSlice(t, glm52.Modalities.Input, []string{"text"})
	assertStringSlice(t, glm52.Modalities.Output, []string{"text"})
}

func TestNewModelDetails(t *testing.T) {
	for _, tc := range []struct {
		id                      string
		input, output           float64
		maxOutput, maxReasoning int
	}{
		{"deepseek-v4.1-flash", 0.3, 1.2, 384_000, 96_000},
		{"glm-5.3", 1.4, 4.4, 128_000, 128_000},
	} {
		t.Run(tc.id, func(t *testing.T) {
			m := findModel(tc.id)
			assertRanges(t, m.Pricing.Input, []PriceRange{{Start: 0, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: tc.input}})
			assertRanges(t, m.Pricing.Output, []PriceRange{{Start: 0, End: ptr(unlimitedPricingEnd), TokenPriceUSDPerMillion: tc.output}})
			assertLimits(t, m.Limits, Limits{ContextWindowTokens: 1_000_000, MaxInputTokens: 1_000_000, MaxOutputTokens: tc.maxOutput, MaxReasoningTokens: ptr(tc.maxReasoning)})
		})
	}
}

func TestRoutes(t *testing.T) {
	server := httptest.NewServer(routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/models")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("/models returned %d, want %d", response.StatusCode, http.StatusOK)
	}
	if response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type %q", response.Header.Get("Content-Type"))
	}

	var body Catalog
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Provider != "Huawei Cloud" || body.Service != "MaaS" || len(body.Models) != 6 {
		t.Fatalf("unexpected /models body: provider=%q service=%q models=%d", body.Provider, body.Service, len(body.Models))
	}

	healthResponse, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer healthResponse.Body.Close()
	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("/health returned %d, want %d", healthResponse.StatusCode, http.StatusOK)
	}

	missingResponse, err := http.Get(server.URL + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer missingResponse.Body.Close()
	if missingResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("/missing returned %d, want %d", missingResponse.StatusCode, http.StatusNotFound)
	}
}

func findModel(id string) Model {
	for _, model := range catalog.Models {
		if model.ID == id {
			return model
		}
	}
	panic("model not found: " + id)
}

func assertLimits(t *testing.T, got, want Limits) {
	t.Helper()
	if got.ContextWindowTokens != want.ContextWindowTokens {
		t.Fatalf("contextWindowTokens = %d, want %d", got.ContextWindowTokens, want.ContextWindowTokens)
	}
	if got.MaxInputTokens != want.MaxInputTokens {
		t.Fatalf("maxInputTokens = %d, want %d", got.MaxInputTokens, want.MaxInputTokens)
	}
	if got.MaxOutputTokens != want.MaxOutputTokens {
		t.Fatalf("maxOutputTokens = %d, want %d", got.MaxOutputTokens, want.MaxOutputTokens)
	}
	if (got.MaxReasoningTokens == nil) != (want.MaxReasoningTokens == nil) {
		t.Fatalf("maxReasoningTokens = %v, want %v", got.MaxReasoningTokens, want.MaxReasoningTokens)
	}
	if got.MaxReasoningTokens != nil && *got.MaxReasoningTokens != *want.MaxReasoningTokens {
		t.Fatalf("maxReasoningTokens = %d, want %d", *got.MaxReasoningTokens, *want.MaxReasoningTokens)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d values, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("value %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func assertRanges(t *testing.T, got, want []PriceRange) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d ranges, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Start != want[index].Start {
			t.Fatalf("range %d start = %d, want %d", index, got[index].Start, want[index].Start)
		}
		if (got[index].End == nil) != (want[index].End == nil) {
			t.Fatalf("range %d end = %v, want %v", index, got[index].End, want[index].End)
		}
		if got[index].End != nil && *got[index].End != *want[index].End {
			t.Fatalf("range %d end = %d, want %d", index, *got[index].End, *want[index].End)
		}
		if got[index].TokenPriceUSDPerMillion != want[index].TokenPriceUSDPerMillion {
			t.Fatalf("range %d price = %f, want %f", index, got[index].TokenPriceUSDPerMillion, want[index].TokenPriceUSDPerMillion)
		}
	}
}
