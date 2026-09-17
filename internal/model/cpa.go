package model

type CPAProvider struct {
	Name          string        `json:"name"`
	BaseURL       string        `json:"base-url"`
	APIKeyEntries []CPAKeyEntry `json:"api-key-entries"`
	Disabled      bool          `json:"disabled"`
	Models        []CPAModel    `json:"models"`
}

type CPAKeyEntry struct {
	APIKey    string `json:"api-key"`
	AuthIndex string `json:"auth-index,omitempty"`
}

type CPAModel struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type CPASyncLog struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	KeyCount int    `json:"key_count"`
	SyncedAt string `json:"synced_at"`
}

type CPASettings struct {
	Endpoint           string   `json:"endpoint"`
	BearerToken        string   `json:"bearer_token"`
	ProviderName       string   `json:"provider_name"`
	BaseURL            string   `json:"base_url"`
	Models             []string `json:"models"`
	OllamaProviderName string   `json:"ollama_provider_name"`
	OllamaBaseURL      string   `json:"ollama_base_url"`
	OllamaModels       []string `json:"ollama_models"`
}

var DefaultCPAModels = []string{
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"kimi-k2.7-code",
	"kimi-k2.6",
	"kimi-k2.5",
	"glm-5.2",
	"glm-5.1",
	"glm-5",
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-plus",
	"qwen3.5-plus",
	"mimo-v2-pro",
	"mimo-v2-omni",
	"mimo-v2.5-pro",
	"mimo-v2.5",
	"hy3-preview",
}
