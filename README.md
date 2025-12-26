# SRE Agent

基于 blades 框架的 SRE 智能巡检 Agent。

## 功能

- **指标采集**: 从 Prometheus 收集系统和业务指标
- **告警管理**: 从 PagerDuty 获取告警信息
- **日志分析**: 从 OpenSearch 检索日志
- **报告生成**: 自动生成巡检报告
- **健康预测**: 系统健康状况预测

## 快速开始

1. 设置环境变量:
   ```bash
   export OPENAI_API_KEY="sk-..."
   export OPENAI_MODEL="gpt-4"
   export PROMETHEUS_URL="http://localhost:9090"
   export PAGERDUTY_API_KEY="pd-..."
   export OPENSEARCH_URL="http://localhost:9200"
   export OPENSEARCH_USER="admin"
   export OPENSEARCH_PASS="admin"
   ```

2. 运行:
   ```bash
   go run cmd/main.go
   ```
