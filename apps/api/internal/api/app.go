package api

import (
	"net/http"
	"strings"

	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/config"
	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/service"
)

type App struct {
	Server       *http.Server
	RunService   *service.RunService
	Orchestrator service.Orchestrator
}

func NewApp(cfg config.Config) *App {
	workflowService := service.NewWorkflowService(cfg.RepoRoot)
	runService := service.NewRunService(cfg.DBPath, cfg.RepoRoot, workflowService)
	coordinator := service.NewInMemoryCoordinator()
	eventBus := service.NewInMemoryEventBus()
	dispatcher := service.NewNoopDispatcher()
	if strings.TrimSpace(cfg.OpenClawHookURL) != "" || strings.TrimSpace(cfg.OpenClawHookToken) != "" {
		dispatcher = service.NewOpenClawHookDispatcher(service.OpenClawHookDispatcherConfig{
			EndpointURL:         cfg.OpenClawHookURL,
			Token:               cfg.OpenClawHookToken,
			SessionKeyPrefix:    cfg.OpenClawSessionKeyPrefix,
			DefaultAgentID:      cfg.OpenClawDefaultAgentID,
			Model:               cfg.OpenClawModel,
			Thinking:            cfg.OpenClawThinking,
			HTTPTimeoutSeconds:  cfg.OpenClawHTTPTimeoutSeconds,
			AgentTimeoutSeconds: cfg.OpenClawAgentTimeoutSeconds,
		})
	}
	orchestrator := service.NewNoopOrchestrator()
	if strings.TrimSpace(cfg.RedisAddr) != "" {
		coordinator = service.NewRedisCoordinator(cfg.RedisAddr)
		eventBus = service.NewRedisEventBus(cfg.RedisAddr, cfg.RedisStream, cfg.RedisConsumerGrp)
		orchestrator = service.NewRedisStreamOrchestrator(cfg.RedisAddr, cfg.RedisStream, cfg.RedisConsumerGrp, runService.WorkerID(), runService)
	}
	runService = runService.WithCoordinator(coordinator).WithEventBus(eventBus).WithDispatcher(dispatcher)

	return &App{
		Server:       newServerWithRuntime(cfg, workflowService, runService, coordinator, eventBus),
		RunService:   runService,
		Orchestrator: orchestrator,
	}
}
