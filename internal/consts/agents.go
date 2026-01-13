package consts

const (
	AgentNameOrchestrator = "orchestrator"
	AgentNameService      = "service_agent"
	AgentNamePrediction   = "prediction_agent"
	AgentNameReport       = "report_agent"
	AgentNameAnalysis     = "analysis_agent"
	AgentNameGeneral      = "general_agent"
)

// RequiredSubAgents 定义系统必需的子 Agent 列表
// 这些 Agent 中至少需要有一个被启用，系统才能正常运行
var RequiredSubAgents = []string{
	AgentNameService,
	AgentNamePrediction,
	AgentNameReport,
}
