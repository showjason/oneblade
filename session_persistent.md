# AI Agent 扩展功能实现指南

本文档基于 blades 框架，详细说明 Session/History 持久化和上下文压缩的实现逻辑。

---

## 1. Session 和 History 持久化

### 1.1 核心概念

```
Session 结构:
├── ID (string)          # 唯一标识符
├── State (map[string]any)  # 状态数据
└── History ([]*Message)    # 对话历史
```

### 1.2 实现逻辑

**Step 1: 定义持久化接口**

```go
type SessionStore interface {
    // 基础 CRUD 操作
    Save(ctx context.Context, session blades.Session) error
    Load(ctx context.Context, sessionID string) (blades.Session, error)
    Delete(ctx context.Context, sessionID string) error
    
    // 扩展操作（生产环境建议实现）
    Exists(ctx context.Context, sessionID string) (bool, error)
    List(ctx context.Context, opts ListOptions) ([]SessionMeta, error)
    UpdateState(ctx context.Context, sessionID string, state map[string]any) error
}

// 分页选项
type ListOptions struct {
    Offset int
    Limit  int
    UserID string  // 可选：按用户筛选
}

// 会话元数据（用于列表展示）
type SessionMeta struct {
    ID        string
    CreatedAt time.Time
    UpdatedAt time.Time
    UserID    string
}
```

**Step 2: 序列化 Session 数据**

```go
// 序列化结构
type SessionData struct {
    ID        string           `json:"id"`
    State     map[string]any   `json:"state"`
    History   []MessageData    `json:"history"`
    CreatedAt time.Time        `json:"created_at"`
    UpdatedAt time.Time        `json:"updated_at"`
    Version   int              `json:"version"`  // 用于数据结构迁移
}

// 消息数据结构（支持工具调用等扩展）
type MessageData struct {
    Role       string            `json:"role"`
    Text       string            `json:"text"`
    ToolCalls  []ToolCallData    `json:"tool_calls,omitempty"`
    ToolResult *ToolResultData   `json:"tool_result,omitempty"`
    Timestamp  time.Time         `json:"timestamp"`
    Metadata   map[string]any    `json:"metadata,omitempty"`
}

type ToolCallData struct {
    ID       string         `json:"id"`
    Name     string         `json:"name"`
    Args     map[string]any `json:"args"`
}

type ToolResultData struct {
    ToolCallID string `json:"tool_call_id"`
    Content    string `json:"content"`
}

const CurrentDataVersion = 1

// 序列化函数
func serializeSession(session blades.Session) *SessionData {
    history := session.History()
    messages := make([]MessageData, len(history))
    for i, msg := range history {
        messages[i] = MessageData{
            Role:      string(msg.Role),
            Text:      msg.Text(),
            Timestamp: time.Now(),  // 实际应从消息中获取
        }
    }
    return &SessionData{
        ID:        session.ID(),
        State:     session.State(),
        History:   messages,
        UpdatedAt: time.Now(),
        Version:   CurrentDataVersion,
    }
}
```

**Step 3: 使用组合模式实现 Session ID 保持**

> **重要**: `blades.NewSession()` 会自动生成新的 UUID，无法恢复原有 ID。
> 解决方案：使用组合模式封装，只覆盖 `ID()` 方法。

```go
// RestoredSession 通过组合模式保持原有 Session ID
type RestoredSession struct {
    originalID string         // 持久化存储的原始 ID
    inner      blades.Session // 委托给原生实现
}

// NewRestoredSession 从持久化数据恢复 Session
func NewRestoredSession(id string, state blades.State) *RestoredSession {
    return &RestoredSession{
        originalID: id,
        inner:      blades.NewSession(state),
    }
}

// ID 返回原始 Session ID（不是 inner 生成的新 ID）
func (s *RestoredSession) ID() string {
    return s.originalID
}

// 以下方法直接委托给 inner 实现
func (s *RestoredSession) State() blades.State              { return s.inner.State() }
func (s *RestoredSession) SetState(key string, value any)   { s.inner.SetState(key, value) }
func (s *RestoredSession) History() []*blades.Message       { return s.inner.History() }
func (s *RestoredSession) Append(ctx context.Context, msg *blades.Message) error {
    return s.inner.Append(ctx, msg)
}
```

**Step 4: 反序列化并恢复 Session**

```go
func deserializeSession(data *SessionData) (blades.Session, error) {
    // 使用 RestoredSession 保持原有 ID
    session := NewRestoredSession(data.ID, data.State)
    
    // 恢复历史消息
    ctx := context.Background()
    for _, msgData := range data.History {
        msg := reconstructMessage(msgData)
        if msg != nil {
            if err := session.Append(ctx, msg); err != nil {
                return nil, fmt.Errorf("failed to append message: %w", err)
            }
        }
    }
    return session, nil
}

// reconstructMessage 根据角色重建消息对象
func reconstructMessage(msgData MessageData) *blades.Message {
    switch blades.Role(msgData.Role) {
    case blades.RoleUser:
        return blades.UserMessage(msgData.Text)
    case blades.RoleAssistant:
        return blades.AssistantMessage(msgData.Text)
    case blades.RoleSystem:
        return blades.SystemMessage(msgData.Text)
    default:
        // 未知角色，记录警告日志
        log.Printf("unknown message role: %s", msgData.Role)
        return nil
    }
}
```

### 1.3 使用方式

```go
// 保存会话
store := NewFileSessionStore("./sessions")
err := store.Save(ctx, session)

// 加载会话
session, err := store.Load(ctx, "session-id")

// 在 Runner 中使用加载的 session
runner.Run(ctx, input, blades.WithSession(session))
```

### 1.4 生产环境建议

| 存储方式 | 适用场景 |
|---------|---------|
| 文件存储 | 开发测试、单机部署 |
| Redis | 分布式、高性能、支持 TTL |
| PostgreSQL | 需要查询历史、合规要求 |
| MongoDB | 灵活 schema、大量历史 |

---

## 2. 上下文压缩

### 2.1 为什么需要上下文压缩？

- **Token 限制**: LLM 有上下文窗口限制（如 GPT-4 的 128K）
- **成本控制**: 更长的上下文 = 更高的 API 费用
- **防止幻觉**: 过长的上下文可能导致模型"迷失"主题

### 2.2 压缩策略对比

| 策略 | 实现复杂度 | 信息保留 | 适用场景 |
|------|-----------|---------|---------|
| 轮次截断 | ⭐ 简单 | 中 | 一般对话 |
| Token 截断 | ⭐⭐ 中等 | 中 | 需要精确控制成本 |
| 摘要压缩 | ⭐⭐⭐ 复杂 | 高 | 需要保留历史上下文 |

### 2.3 轮次压缩实现

**核心逻辑**: 保留最近 N 轮对话

```go
func TurnCompression(maxTurns int) blades.Middleware {
    return func(next blades.Handler) blades.Handler {
        return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
            session, ok := blades.FromSessionContext(ctx)
            if !ok {
                return next.Handle(ctx, inv)
            }
            
            history := session.History()
            
            // 计算当前轮数（一个 user + assistant = 1 轮）
            turns := 0
            for _, msg := range history {
                if msg.Role == blades.RoleUser {
                    turns++
                }
            }
            
            // 如果超过限制，截取最近的 N 轮
            if turns > maxTurns {
                cutoff := findCutoffIndex(history, maxTurns)
                inv.History = history[cutoff:]
            } else {
                inv.History = append(inv.History, history...)
            }
            
            return next.Handle(ctx, inv)
        })
    }
}

// 找到保留 maxTurns 轮的截断点
func findCutoffIndex(history []*blades.Message, maxTurns int) int {
    userCount := 0
    for i := len(history) - 1; i >= 0; i-- {
        if history[i].Role == blades.RoleUser {
            userCount++
            if userCount > maxTurns {
                return i + 1
            }
        }
    }
    return 0
}
```

### 2.4 Token 压缩实现

**核心逻辑**: 限制总 token 数量

```go
func TokenCompression(maxTokens int) blades.Middleware {
    return func(next blades.Handler) blades.Handler {
        return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
            session, ok := blades.FromSessionContext(ctx)
            if !ok {
                return next.Handle(ctx, inv)
            }
            
            history := session.History()
            totalTokens := estimateTokens(history)
            
            if totalTokens > maxTokens {
                // 从后往前保留，直到达到限制
                var kept []*blades.Message
                tokens := 0
                for i := len(history) - 1; i >= 0; i-- {
                    msgTokens := estimateTokens([]*blades.Message{history[i]})
                    if tokens + msgTokens > maxTokens {
                        break
                    }
                    tokens += msgTokens
                    kept = append([]*blades.Message{history[i]}, kept...)
                }
                inv.History = kept
            } else {
                inv.History = append(inv.History, history...)
            }
            
            return next.Handle(ctx, inv)
        })
    }
}

// Token 估算（支持中英文混合）
// 注意：英文约 4 字符 = 1 token，中文约 1 字符 = 1.5-2 tokens
func estimateTokens(messages []*blades.Message) int {
    total := 0
    for _, msg := range messages {
        total += estimateTextTokens(msg.Text())
    }
    return total
}

// estimateTextTokens 对中英文混合文本进行 token 估算
func estimateTextTokens(text string) int {
    var tokens int
    for _, r := range text {
        if r > 127 {
            // 非 ASCII 字符（中文、日文等）：约 1.5-2 tokens
            tokens += 2
        } else {
            // ASCII 字符：约 4 字符 = 1 token
            tokens++
        }
    }
    // ASCII 部分按 4:1 换算
    return tokens / 4
}

// 生产环境建议：使用 tiktoken 库进行精确计算
// import "github.com/pkoukk/tiktoken-go"
// func estimateTokensExact(text, model string) int {
//     enc, _ := tiktoken.EncodingForModel(model)
//     return len(enc.Encode(text, nil, nil))
// }
```

### 2.5 摘要压缩实现（高级）

**核心逻辑**: 使用 LLM 生成历史摘要

```go
type Summarizer interface {
    Summarize(ctx context.Context, messages []*blades.Message) (string, error)
}

func SummaryCompression(maxTurns int, summarizer Summarizer) blades.Middleware {
    return func(next blades.Handler) blades.Handler {
        return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
            session, ok := blades.FromSessionContext(ctx)
            if !ok {
                return next.Handle(ctx, inv)
            }
            
            history := session.History()
            turns := countTurns(history)
            
            if turns > maxTurns {
                cutoff := findCutoffIndex(history, maxTurns)
                oldMessages := history[:cutoff]
                recentMessages := history[cutoff:]
                
                // 生成摘要
                summary, err := summarizer.Summarize(ctx, oldMessages)
                if err == nil && summary != "" {
                    summaryMsg := blades.AssistantMessage("[历史摘要] " + summary)
                    inv.History = append([]*blades.Message{summaryMsg}, recentMessages...)
                } else {
                    inv.History = recentMessages
                }
            } else {
                inv.History = append(inv.History, history...)
            }
            
            return next.Handle(ctx, inv)
        })
    }
}

// LLM 摘要器实现示例
type LLMSummarizer struct {
    model blades.ModelProvider
}

func (s *LLMSummarizer) Summarize(ctx context.Context, messages []*blades.Message) (string, error) {
    prompt := "请将以下对话历史压缩成一段简短的摘要，保留关键信息：\n\n"
    for _, msg := range messages {
        prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Text())
    }
    
    resp, err := s.model.Generate(ctx, &blades.ModelRequest{
        Messages: []*blades.Message{blades.UserMessage(prompt)},
    })
    if err != nil {
        return "", err
    }
    return resp.Message.Text(), nil
}
```

### 2.6 中间件使用方式

```go
agent, _ := blades.NewAgent(
    "MyAgent",
    blades.WithModel(model),
    blades.WithMiddlewares(
        TokenCompression(4000),     // 最大 4000 tokens
        TurnCompression(10),        // 最多 10 轮
        // 或使用框架自带的
        middleware.ConversationBuffered(20),  // 最多 20 条消息
    ),
)
```

---

## 3. 完整使用示例

### 3.1 基础使用

```go
func main() {
    ctx := context.Background()
    
    // 1. 创建持久化存储
    store := NewFileSessionStore("./sessions")
    
    // 2. 创建或加载 session（包含错误处理）
    var session blades.Session
    existingID := loadLastSessionID()
    if existingID != "" {
        var err error
        session, err = store.Load(ctx, existingID)
        if err != nil {
            log.Printf("Failed to load session %s: %v, creating new session", existingID, err)
            session = blades.NewSession()
        }
    } else {
        session = blades.NewSession()
    }
    
    // 3. 创建带压缩中间件的 agent
    agent, err := blades.NewAgent(
        "Orchestrator",
        blades.WithModel(model),
        blades.WithMiddlewares(
            TokenCompression(4000),
            TurnCompression(10),
        ),
    )
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    
    // 4. 多轮对话循环
    runner := blades.NewRunner(agent)
    for {
        input := getUserInput()
        if input == "exit" {
            break
        }
        
        // 使用流式输出处理函数
        if err := handleStreamingResponse(ctx, runner, session, input); err != nil {
            log.Printf("Error during conversation: %v", err)
            continue
        }
        
        // 保存 session（带错误处理）
        if err := store.Save(ctx, session); err != nil {
            log.Printf("Failed to save session: %v", err)
        }
    }
}
```

### 3.2 Streaming 响应处理与持久化

> **重要**: Streaming 过程中通常**不保存** Session，应在完整响应生成后统一保存。

```go
// handleStreamingResponse 处理流式响应并在完成后更新 Session
func handleStreamingResponse(
    ctx context.Context,
    runner *blades.Runner,
    session blades.Session,
    input string,
) error {
    // 流式输出
    stream := runner.RunStream(ctx, blades.UserMessage(input), 
        blades.WithSession(session))
    
    // 累积完整响应
    var fullResponse strings.Builder
    var streamErr error
    
    for output, err := range stream {
        if err != nil {
            streamErr = err
            break
        }
        chunk := output.Text()
        fullResponse.WriteString(chunk)
        fmt.Print(chunk)  // 实时输出到终端
    }
    fmt.Println()
    
    // 如果流式传输中断，返回错误
    if streamErr != nil {
        return fmt.Errorf("stream interrupted: %w", streamErr)
    }
    
    // 注意：blades.Runner 通常会自动将响应追加到 Session
    // 如果框架不自动处理，可手动追加：
    // session.Append(ctx, blades.AssistantMessage(fullResponse.String()))
    
    return nil
}
```

### 3.3 生产环境完整示例

```go
func runProductionAgent() error {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    
    // 配置
    cfg := &Config{
        StorePath:   "./sessions",
        MaxTokens:   4000,
        MaxTurns:    10,
        AutoSave:    true,
        SaveOnExit:  true,
    }
    
    // 创建存储（使用 Redis 存储用于分布式场景）
    store, err := NewRedisSessionStore(cfg.RedisURL)
    if err != nil {
        return fmt.Errorf("failed to create store: %w", err)
    }
    defer store.Close()
    
    // 创建 Agent
    agent, err := blades.NewAgent(
        "ProductionOrchestrator",
        blades.WithModel(model),
        blades.WithMiddlewares(
            TokenCompression(cfg.MaxTokens),
            TurnCompression(cfg.MaxTurns),
        ),
    )
    if err != nil {
        return fmt.Errorf("failed to create agent: %w", err)
    }
    
    // 加载或创建 Session
    session, err := loadOrCreateSession(ctx, store)
    if err != nil {
        return err
    }
    
    // 确保退出时保存
    if cfg.SaveOnExit {
        defer func() {
            if err := store.Save(context.Background(), session); err != nil {
                log.Printf("Failed to save session on exit: %v", err)
            }
        }()
    }
    
    // 运行对话循环
    runner := blades.NewRunner(agent)
    return runConversationLoop(ctx, runner, session, store, cfg)
}

// loadOrCreateSession 根据请求获取或创建 Session
func loadOrCreateSession(ctx context.Context, store SessionStore, sessionID string) (blades.Session, error) {
    if sessionID != "" {
        exists, err := store.Exists(ctx, sessionID)
        if err != nil {
            return nil, fmt.Errorf("failed to check session: %w", err)
        }
        if exists {
            session, err := store.Load(ctx, sessionID)
            if err != nil {
                return nil, fmt.Errorf("failed to load session: %w", err)
            }
            log.Printf("Loaded existing session: %s", sessionID)
            return session, nil
        }
    }
    
    session := blades.NewSession()
    log.Printf("Created new session: %s", session.ID())
    return session, nil
}
```

### 3.4 Web 服务/多用户场景示例（重要）

> **注意**: 在 Web 服务中，必须确保每个用户/请求使用**独立的 Session**，切勿在全局共享同一个 Session 实例。

```go
// HTTP Handler 示例
func ChatHandler(store SessionStore, runner *blades.Runner) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        // 1. 从 Header 或 Cookie 获取 Session ID
        sessionID := r.Header.Get("X-Session-ID")
        
        // 2. 加载该用户的独立 Session
        session, err := loadOrCreateSession(ctx, store, sessionID)
        if err != nil {
            http.Error(w, "Session error", http.StatusInternalServerError)
            return
        }
        
        // 3. 获取用户输入
        var req ChatRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }
        
        // 4. 设置响应 Header（返回 Session ID）
        w.Header().Set("X-Session-ID", session.ID())
        w.Header().Set("Content-Type", "text/event-stream")
        
        // 5. 使用该用户自己的 Session 运行（关键点：WithSession）
        stream := runner.RunStream(ctx, blades.UserMessage(req.Message), 
            blades.WithSession(session))
            
        // 6. 流式处理并累积完整响应
        var fullResponse strings.Builder
        for output, err := range stream {
            if err != nil {
                log.Printf("Stream error: %v", err)
                break
            }
            chunk := output.Text()
            fullResponse.WriteString(chunk)
            fmt.Fprintf(w, "data: %s\n\n", chunk)
            w.(http.Flusher).Flush()
        }
        
        // 7. 保存该用户的 Session
        // 注意：blades Runner 通常已追加了 Assistant 消息，这里只需保存
        if err := store.Save(ctx, session); err != nil {
            log.Printf("Failed to save session %s: %v", session.ID(), err)
        }
    }
}
```

---

## 4. 关键要点总结

### Session 持久化

1. **Session ID 保持**: 使用 `RestoredSession` 组合模式保持原有 ID
2. **序列化/反序列化**: 将 Session 转换为可存储的 `SessionData` 格式
3. **存储后端**: 根据需求选择文件/Redis/数据库
4. **加载恢复**: 在 Runner 中通过 `WithSession` 使用恢复的 Session

### 上下文压缩

1. **中间件模式**: 通过 `WithMiddlewares` 注入压缩逻辑
2. **策略选择**: 轮次/Token/摘要，根据场景选择
3. **执行时机**: 在 session history 传递给 LLM 之前压缩
4. **Token 估算**: 使用多语言感知的估算方法（中文 ≈ 2 tokens/字符）

### Streaming 处理

1. **流式输出不立即保存**: 累积完整响应后再更新 Session
2. **错误处理**: 妥善处理流式传输中断的情况
3. **退出保存**: 使用 `defer` 确保程序退出时保存最新状态

### 最佳实践

- Session 只在入口（Runner）注入一次
- 压缩中间件从 Context 获取 Session
- SubAgent 通过 `NewAgentTool` 作为工具调用时共享 Session
- 所有存储操作都应包含错误处理
- 生产环境建议使用 Redis 或数据库存储

