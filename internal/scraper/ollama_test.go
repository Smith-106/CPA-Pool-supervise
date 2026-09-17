package scraper

import (
	"encoding/json"
	"testing"
)

func TestFetchOllamaUsageParsing(t *testing.T) {
	var raw ollamaUsageResponse
	if err := json.Unmarshal([]byte(`{"limits":{"monthly":{"usage":0.356,"models":[{"name":"gemma4:31b","request_count":470},{"name":"gpt-oss:20b","request_count":6}]}}}`), &raw); err != nil {
		t.Fatal(err)
	}
	if pctInt(raw.Limits.Monthly.Usage) != 36 {
		t.Fatalf("monthly percent = %d, want 36", pctInt(raw.Limits.Monthly.Usage))
	}
	if len(raw.Limits.Monthly.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(raw.Limits.Monthly.Models))
	}
	if raw.Limits.Monthly.Models[0].Name != "gemma4:31b" || raw.Limits.Monthly.Models[0].RequestCount != 470 {
		t.Fatalf("model entry wrong: %+v", raw.Limits.Monthly.Models[0])
	}
}
