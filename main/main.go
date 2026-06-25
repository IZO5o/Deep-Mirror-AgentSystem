package main

import (
	"context"
	"path/filepath"

	"github.com/joho/godotenv"

	"agent-web-base/agent"
	ctxengine "agent-web-base/agent/context"
	"agent-web-base/agent/memory"
	"agent-web-base/agent/storage"
	"agent-web-base/agent/tool"
	"agent-web-base/server"
	"agent-web-base/shared"
	"agent-web-base/shared/log"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	appConf, err := shared.LoadAppConfig("config.json")
	if err != nil {
		log.Errorf("Failed to load config.json: %v", err)
		panic(err)
	}

	mcpServerMap, err := shared.LoadMcpServerConfig("mcp-server.json")
	if err != nil {
		log.Warnf("Failed to load MCP server configuration: %v", err)
	}
	mcpClients := make([]*agent.McpClient, 0)
	for name, config := range mcpServerMap {
		mcpClient := agent.NewMcpToolProvider(name, config)
		if err := mcpClient.RefreshTools(ctx); err != nil {
			log.Warnf("Failed to refresh tools for MCP server %s: %v", name, err)
			continue
		}
		mcpClients = append(mcpClients, mcpClient)
	}

	offloadStorage := storage.NewMemoryStorage()
	summarizer := ctxengine.NewLLMSummarizer(appConf.LLMProviders.BackModel, 200)
	policies := []ctxengine.Policy{
		ctxengine.NewOffloadPolicy(offloadStorage, 0.4, 0, 100),
		ctxengine.NewSummaryPolicy(summarizer, 10, 20, 0.6),
		ctxengine.NewTruncatePolicy(0, 0.85),
	}

	stateDir := filepath.Join(shared.GetWorkspaceDir(), ".agent-web-base")
	globalStorage := storage.NewFileSystemStorage(filepath.Join(stateDir, "global"))
	workspaceStorage := storage.NewFileSystemStorage(filepath.Join(stateDir, "workspace"))
	memoryUpdater := memory.NewLLMMemoryUpdater(appConf.LLMProviders.BackModel)
	multiLevelMemory := memory.NewMultiLevelMemory(globalStorage, workspaceStorage, memoryUpdater)

	confirmConfig := agent.ToolConfirmConfig{
		RequireConfirmTools: map[tool.AgentTool]bool{
			tool.AgentToolBash: true,
		},
	}

	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.CreateBashTool(shared.GetWorkspaceDir()),
		tool.NewLoadStorageTool(offloadStorage),
		tool.NewLoadSkillTool(),
	}

	db, err := server.InitDB("agent-web-base.db")
	if err != nil {
		log.Errorf("Failed to initialize database: %v", err)
		panic(err)
	}

	agentRegistry, err := agent.NewAgentRegistry(
		agent.AgentTypeAssistant,
		agent.DefaultAgentProfiles(),
		func(profile agent.AgentProfile) agent.Runner {
			contextEngine := ctxengine.NewContextEngine(multiLevelMemory, policies)
			return agent.NewAgent(
				appConf.LLMProviders.FrontModel,
				profile.SystemPrompt,
				confirmConfig,
				tools,
				mcpClients,
				contextEngine,
			)
		},
	)
	if err != nil {
		log.Errorf("Failed to initialize agent registry: %v", err)
		panic(err)
	}

	s := server.NewServer(db, agentRegistry)
	mediaDir := appConf.Media.StorageDir
	if mediaDir == "" {
		mediaDir = filepath.Join(stateDir, "media")
	}
	s.ConfigureTranscription(
		mediaDir,
		server.NewOpenAIASRClient(appConf.ASR.BaseURL, appConf.ASR.ApiKey, appConf.ASR.Model),
		server.NewFFmpegAudioExtractor(appConf.Media.FFmpegPath),
	)
	s.StartTranscriptionWorker(ctx)

	router := server.NewRouter(s)

	if err := router.Run(":8080"); err != nil {
		log.Errorf("Server failed: %v", err)
		panic(err)
	}
}
