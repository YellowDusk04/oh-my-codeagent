# 第三节：多轮对话与 Session 记忆

> 💡 **回溯代码状态：** `git checkout 044f44d` 回到本次提交，`git checkout main` 回到最新。

---

## 为什么需要 Session？

第一节的 Demo 只能进行单轮对话：每次运行都是全新的开始，Agent 不记得你上一句说了什么。

要让 Agent 能"记住"之前的对话，就需要 **Session（会话）**。Session 负责保存对话历史，这样多轮对话才能连贯。

我们选用 **SQLite** 作为 Session 的存储后端——轻量、无需额外安装数据库服务，适合本地开发。

---

## 改造概览

这一节我们做了三件事：

1. **引入多轮对话循环**：用 `for` 循环让程序持续读取用户输入
2. **接入 SQLite Session**：把对话历史持久化到本地文件
3. **支持 Session 切换**：用 `/session` 命令可以新建或切换会话

改造后的文件结构：

```
oh-my-codeagent/
├── main.go        # 主入口，负责启动和协调
├── agent.go       # Agent 和 Runner 的创建
├── session.go     # SQLite Session Service 的创建
└── tracing.go     # Langfuse 链路追踪
```

---

## 第一步：创建 session.go

新增 `session.go`，负责创建 SQLite Session Service：

```go
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/sqlite"
)

func createSessionService() (session.Service, error) {
	dsn := "file:sessions.db?_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	svc, err := sqlite.NewService(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("创建 session service 失败: %w", err)
	}

	return svc, nil
}
```

**这段代码在干嘛？**

1. **`sql.Open("sqlite3", dsn)`**：打开（或创建）SQLite 数据库文件 `sessions.db`
2. **`db.SetMaxOpenConns(1)`**：SQLite 建议单连接，避免锁冲突
3. **`sqlite.NewService(db)`**：用这个数据库连接创建一个 Session Service

> 💡 **小提示**：`sessions.db` 文件会自动保存在当前目录，里面存的就是对话历史。

---

## 第二步：改造 agent.go

把 Session Service 传给 Runner，让 Agent 能读取对话历史：

```go
const (
	agentName = "code-agent"
	appName   = "oh-my-codeagent"
)

func createRunner(sessionService session.Service) (runner.Runner, error) {
	modelInstance := openai.New(
		os.Getenv("MODEL_ID"),
		openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		openai.WithBaseURL(os.Getenv("BASE_URL")),
	)

	enableThinking := os.Getenv("ENABLE_THINKING") == "true"

	agent := llmagent.New(agentName,
		llmagent.WithModel(modelInstance),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream:          true,
			ThinkingEnabled: &enableThinking,
		}),
	)

	r := runner.NewRunner(appName, agent,
		runner.WithSessionService(sessionService),  // 关键：把 Session Service 传进来
	)

	return r, nil
}
```

**关键改动**：`runner.WithSessionService(sessionService)` —— 这句话告诉 Runner："你用这个 Session Service 来管理对话历史"。

---

## 第三步：改造 main.go，加入多轮对话循环

现在来实现交互式多轮对话：

```go
func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	// 启动 Langfuse 遥测
	clean, err := langfuse.Start(context.Background())
	if err != nil {
		log.Printf("Failed to start Langfuse telemetry: %v", err)
	} else {
		defer func() {
			if err := clean(context.Background()); err != nil {
				log.Printf("Failed to clean up: %v", err)
			}
		}()
	}

	// 创建 session service 和 runner
	sessionService, err := createSessionService()
	if err != nil {
		log.Fatal("创建 session service 失败:", err)
	}

	r, err := createRunner(sessionService)
	if err != nil {
		log.Fatal("创建 runner 失败:", err)
	}

	// 设置用户和会话 ID
	userID := "user-001"
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())

	// 启动多轮对话
	startChat(r, userID, &sessionID)

	// 退出时打印 session ID
	fmt.Printf("\n📝 当前 session ID: %s\n", sessionID)
	fmt.Println("💡 下次启动时可以使用此 ID 恢复会话")
}
```

**`startChat()` 函数**：实现 `while true` 循环，持续读取用户输入并调用 Agent：

```go
func startChat(r runner.Runner, userID string, sessionID *string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("🤖 多轮对话已启动！\n")
	fmt.Printf("📝 当前 session ID: %s\n", *sessionID)
	fmt.Println("💡 命令: /exit 退出, /session new 新建会话, /session <id> 切换会话")

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		userInput := scanner.Text()
		if userInput == "" {
			continue
		}

		// 退出命令
		if userInput == "/exit" {
			fmt.Println("👋 再见！")
			break
		}

		// 处理 /session 命令
		if strings.HasPrefix(userInput, "/session") {
			handleSessionCommand(userInput, sessionID)
			continue
		}

		// 创建带 baggage 的 context
		ctx := NewBaggageContext(userID, *sessionID)

		events, err := r.Run(ctx,
			userID,
			*sessionID,
			model.NewUserMessage(userInput),
		)
		if err != nil {
			log.Println("运行失败:", err)
			continue
		}

		// 处理事件流
		fmt.Print("Assistant: ")
		for event := range events {
			if event != nil && len(event.Choices) > 0 {
				choice := event.Choices[0]
				if choice.Delta.Content != "" {
					fmt.Print(choice.Delta.Content)
				}
			}
		}
		fmt.Println()
	}
}
```

---

## 第四步：支持 Session 切换

实现 `/session` 命令，让用户可以新建或切换会话：

```go
func handleSessionCommand(input string, sessionID *string) {
	parts := strings.Fields(input)

	if len(parts) == 1 {
		// /session - 显示当前 session ID
		fmt.Printf("📝 当前 session ID: %s\n", *sessionID)
		return
	}

	if len(parts) == 2 && parts[1] == "new" {
		// /session new - 创建新会话
		*sessionID = fmt.Sprintf("session-%d", time.Now().Unix())
		fmt.Printf("✨ 已创建新会话\n")
		fmt.Printf("📝 新 session ID: %s\n", *sessionID)
		return
	}

	if len(parts) == 2 {
		// /session <id> - 切换到指定会话
		*sessionID = parts[1]
		fmt.Printf("🔄 已切换到会话: %s\n", *sessionID)
		return
	}

	fmt.Println("❌ 用法: /session | /session new | /session <id>")
}
```

**支持的命令**：

| 命令 | 作用 |
|------|------|
| `/session` | 显示当前 session ID |
| `/session new` | 创建新会话（清空对话历史） |
| `/session <id>` | 切换到指定会话（恢复对话历史） |
| `/exit` | 退出程序（会打印当前 session ID） |

---

## 跑一下

```bash
go run .
```

**运行效果**：

```
🤖 多轮对话已启动！
📝 当前 session ID: session-1749707823
💡 命令: /exit 退出, /session new 新建会话, /session <id> 切换会话

You: 你好
Assistant: 你好！有什么我可以帮助你的吗？

You: 你还记得我刚才说了什么吗？
Assistant: 当然记得！你刚才说"你好"。

You: /session new
✨ 已创建新会话
📝 新 session ID: session-1749707956

You: 我刚才说了什么？
Assistant: 你好！我是 code-agent，有什么可以帮助你的吗？

You: /exit
👋 再见！

📝 当前 session ID: session-1749707956
💡 下次启动时可以使用此 ID 恢复会话
```

---

## 数据存在哪？

对话历史保存在 `sessions.db` 文件里（SQLite 格式）。你可以用任何 SQLite 客户端打开它，看看里面存了什么。

```
oh-my-codeagent/
├── main.go
├── agent.go
├── session.go
├── tracing.go
└── sessions.db    ← 对话历史存在这里
```

---

## 小结

这一节我们改了四个文件：

1. **`session.go`** 新建，负责创建 SQLite Session Service
2. **`agent.go`** 改造，把 Session Service 传给 Runner
3. **`main.go`** 改造，加入多轮对话循环和 `/session` 命令
4. **`tracing.go`** 不变

现在 Agent 能记住对话历史了，多轮对话变得连贯自然。

下一节，我们教 Agent 用工具（Tool）。
