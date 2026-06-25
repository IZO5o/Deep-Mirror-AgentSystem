package shared

import (
	"encoding/json"
	"os"
)

type AppConfig struct {
	LLMProviders struct {
		FrontModel ModelConfig `json:"front_model"`
		BackModel  ModelConfig `json:"back_model"`
	} `json:"llm_providers"`
	Media MediaConfig `json:"media"`
	ASR   ASRConfig   `json:"asr"`
}

type ModelConfig struct {
	BaseURL string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Model   string `json:"model"`

	ContextWindow int `json:"context_window"`
}

type MediaConfig struct {
	StorageDir string `json:"storage_dir"`
	FFmpegPath string `json:"ffmpeg_path"`
}

type ASRConfig struct {
	BaseURL string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func NewModelConfig() ModelConfig {
	return ModelConfig{
		BaseURL:       getEnvDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		ApiKey:        getEnvDefault("OPENAI_API_KEY", ""),
		Model:         getEnvDefault("OPENAI_MODEL", "gpt-5.2"),
		ContextWindow: 200000,
	}
}

func NewASRConfig() ASRConfig {
	return ASRConfig{
		BaseURL: getEnvDefault("ASR_BASE_URL", getEnvDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")),
		ApiKey:  getEnvDefault("ASR_API_KEY", getEnvDefault("OPENAI_API_KEY", "")),
		Model:   getEnvDefault("ASR_MODEL", "gpt-4o-transcribe"),
	}
}

func NewMediaConfig() MediaConfig {
	return MediaConfig{
		StorageDir: getEnvDefault("MEDIA_STORAGE_DIR", ""),
		FFmpegPath: getEnvDefault("FFMPEG_PATH", "ffmpeg"),
	}
}

func getEnvDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func LoadAppConfig(path string) (AppConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return AppConfig{
			LLMProviders: struct {
				FrontModel ModelConfig `json:"front_model"`
				BackModel  ModelConfig `json:"back_model"`
			}{
				FrontModel: NewModelConfig(),
				BackModel:  NewModelConfig(),
			},
			Media: NewMediaConfig(),
			ASR:   NewASRConfig(),
		}, nil
	}
	var config AppConfig
	err = json.Unmarshal(content, &config)
	if err != nil {
		return AppConfig{}, err
	}
	if config.Media.FFmpegPath == "" {
		config.Media.FFmpegPath = getEnvDefault("FFMPEG_PATH", "ffmpeg")
	}
	if config.Media.StorageDir == "" {
		config.Media.StorageDir = getEnvDefault("MEDIA_STORAGE_DIR", "")
	}
	if config.ASR.BaseURL == "" {
		config.ASR.BaseURL = getEnvDefault("ASR_BASE_URL", config.LLMProviders.BackModel.BaseURL)
	}
	if config.ASR.ApiKey == "" {
		config.ASR.ApiKey = getEnvDefault("ASR_API_KEY", config.LLMProviders.BackModel.ApiKey)
	}
	if config.ASR.Model == "" {
		config.ASR.Model = getEnvDefault("ASR_MODEL", "gpt-4o-transcribe")
	}
	return config, nil
}
