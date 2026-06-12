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

新增 `session.go`，负责创建 SQLite Session Service。

**核心代码**（只展示关键部分）：

```go
func createSessionService() (session.Service, error) {
	dsn := "file:sessions.db?_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	// ... 错误处理

	db.SetMaxOpenConns(1)  // SQLite 建议单连接
	db.SetMaxIdleConns(1)

	svc, err := sqlite.NewService(db)
	// ... 错误处理

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

把 Session Service 传给 Runner，让 Agent 能读取对话历史。

**关键改动**：

```go
r := runner.NewRunner(appName, agent,
	runner.WithSessionService(sessionService),  // 关键：把 Session Service 传进来
)
```

**为什么要这样改？**

- `runner.WithSessionService(sessionService)` 告诉 Runner："你用这个 Session Service 来管理对话历史"
- 这样每次调用 `r.Run()` 时，Runner 会自动从 Session 中读取历史消息，并把新消息保存回去

---

## 第三步：改造 main.go，加入多轮对话循环

现在来实现交互式多轮对话。

**核心逻辑**：

1. **初始化**：创建 Session Service 和 Runner
2. **启动循环**：用 `for` 循环持续读取用户输入
3. **处理命令**：支持 `/exit` 退出、`/session` 切换会话
4. **调用 Agent**：把用户输入传给 `r.Run()`，并打印流式响应

**关键代码片段**：

```go
func startChat(r runner.Runner, userID string, sessionID *string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("🤖 多轮对话已启动！")

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

		// 调用 Agent
		ctx := NewBaggageContext(userID, *sessionID)
		events, err := r.Run(ctx, userID, *sessionID, model.NewUserMessage(userInput))
		// ... 处理事件流
	}
}
```

**这段代码在干嘛？**

- **`for` 循环**：实现 `while true` 效果，持续读取用户输入
- **`/exit` 命令**：退出循环，结束程序
- **`/session` 命令**：调用 `handleSessionCommand()` 处理会话切换
- **`r.Run()`**：把用户输入传给 Agent，返回事件流（stream）
- **事件流处理**：逐块打印 Agent 的回复（`choice.Delta.Content`）

---

## 第四步：支持 Session 切换

实现 `/session` 命令，让用户可以新建或切换会话。

**支持的命令**：

| 命令 | 作用 |
|------|------|
| `/session` | 显示当前 session ID |
| `/session new` | 创建新会话（清空对话历史） |
| `/session <id>` | 切换到指定会话（恢复对话历史） |
| `/exit` | 退出程序（会打印当前 session ID） |

**实现逻辑**：

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

**关键点**：

- 使用指针 `*sessionID` 来修改外部的 session ID
- `/session new` 会生成新的 session ID（基于时间戳）
- `/session <id>` 会切换到指定的 session ID，从而恢复之前的对话历史

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

You: 我会 Golang
Assistant: 太好了！Golang 是一门很棒的语言。有什么我可以帮你的吗？

You: 我会什么？
Assistant: 你刚才告诉我你会 Golang。

You: /session new
✨ 已创建新会话
📝 新 session ID: session-1749707956

You: 我会什么？
Assistant: 你好！我是 code-agent，有什么可以帮助你的吗？

You: /exit
👋 再见！

📝 当前 session ID: session-1749707956
💡 下次启动时可以使用此 ID 恢复会话
```

**这个例子说明了什么？**

1. **第一轮**：告诉 Agent "我会 Golang"，Agent 确认了
2. **第二轮**：问 Agent "我会什么？"，Agent 能记住刚才说的话（因为 Session 保存了对话历史）
3. **切换会话**：用 `/session new` 创建新会话后，Agent 不记得之前的信息（因为新会话的 Session 是空的）
4. **恢复会话**：下次启动时，用 `/session session-1749707823` 可以恢复到第一个会话，Agent 会记得你说过"我会 Golang"

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

**Session 里面存了什么？**

- **消息历史**：用户和 Agent 的所有对话（包括时间戳）
- **Session 元数据**：session ID、user ID、创建时间等
- **状态信息**：如果有工具调用，也会保存在 Session 里

---

## 小结

这一节我们改了四个文件：

1. **`session.go`** 新建，负责创建 SQLite Session Service
2. **`agent.go`** 改造，把 Session Service 传给 Runner
3. **`main.go`** 改造，加入多轮对话循环和 `/session` 命令
4. **`tracing.go`** 不变

**核心要点**：

- Session 让 Agent 能记住对话历史，实现多轮对话
- SQLite 是一个轻量的本地存储方案，适合开发环境
- `/session` 命令让用户可以灵活管理会话

现在 Agent 能记住对话历史了，多轮对话变得连贯自然。

下一节，我们教 Agent 用工具（Tool）。
