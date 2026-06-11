# 第一节：写出第一个能跑的 Code Agent

本项目使用 [trpc-agent-go](https://github.com/trpc-group/trpc-agent-go) 框架进行开发。

> 💡 **阅读本文时，如何回到当前代码状态？**
> 
> 随着项目迭代，代码会不断更新。如果你在阅读本文时，想看到我们当前这个"最小 Demo"的代码状态，可以运行下面这条命令：
> 
> ```bash
> git checkout d8b1658
> ```
> 
> 这条命令会把代码"回溯"到我们这次提交时的状态。等你看得差不多了，想回到最新的代码，再运行 `git checkout main` 就行了。

---

好，我们先不扯那么多，直接来写一个能运行的最小的 Demo。这样你马上就能看到效果，成就感满满。

先来看看我们最终能达到的效果（终端运行效果）：

```bash
% go run .
你好！有什么我可以帮忙的吗？
```

看到没？我们只需要几行代码，就能让模型跟我们对话了！

接下来我会一步步教你怎么做。准备好了吗？我们开始吧！

---

## 第一步：先把 Model 定义出来

要写一个 Agent，首先得告诉它："你用哪个大模型？" 这一步就是定义 Model。

不过，在写代码之前，我们得先搞清楚一个问题：**这些配置信息从哪来？**

你看，待会儿我们的代码里会出现 `os.Getenv("MODEL_ID")` 这样的写法。这个 `os.Getenv()` 是用来读取"环境变量"的。那这些环境变量又是从哪设置的呢？

答案就是：**`.env` 文件**。

### 先说说 .env 文件

`.env` 文件用来存放敏感信息（比如 API 密钥、模型 ID 等），这样我们就不用把敏感信息直接写在代码里了。

看看我们的 `.env` 文件长啥样：

```env
OPENAI_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
BASE_URL=https://maas-api.cn-huabei-1.xf-yun.com/v2
MODEL_ID=xxxxxxxxxxxxxxxx
```

> ⚠️ **注意**：我这里把所有的值都用 `xxxx` 替代了。实际情况下，这里应该填真实的值。**这个文件里的信息千万不能给别人看，不然别人就能用你的账号调用模型了。**

为什么要用 `.env` 文件？因为如果你把敏感信息直接写在代码里，然后不小心把代码传到了 GitHub，那你的密钥就泄露了。而 `.env` 文件通常会被加到 `.gitignore` 里，不会被提交到 Git，这样就安全多了。

然后，在我们的 `main.go` 的开头，有这么一段代码：

```go
err := godotenv.Load()
if err != nil {
    fmt.Println("加载 .env 文件失败:", err)
}
```

这段代码的作用是：把 `.env` 文件里的键值对加载成"环境变量"。这样之后，我们就可以通过 `os.Getenv("MODEL_ID")` 这样的方式，把配置读出来了。

好了，配置的问题搞清楚了。现在我们来写代码，定义 Model：

```go
modelInstance := openai.New(os.Getenv("MODEL_ID"),
    openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    openai.WithBaseURL(os.Getenv("BASE_URL")),
)
```

**这段代码在干嘛？**

我们调用了 `openai.New()` 这个函数，创建了一个 Model 实例。这个函数接收三个东西：

1. **第一个参数**：`os.Getenv("MODEL_ID")` —— 这就是你要用的模型的名字。
2. **第二个参数**：`openai.WithAPIKey(...)` —— 这是你的 API 密钥，用来证明"你有权调用这个模型"。
3. **第三个参数**：`openai.WithBaseURL(...)` —— 这是模型服务的地址。因为我们用的是星火大模型的 API，所以这里填的是 `https://maas-api.cn-huabei-1.xf-yun.com/v2`。

> 💡 **小提示**：你可能会问，"为什么不用 OpenAI 官方的？" 其实这个框架设计得很灵活，`openai.New()` 这个函数是按照 OpenAI 的 API 标准来的，所以任何兼容 OpenAI API 的服务都能用，不一定是 OpenAI 自家的。

---

## 第二步：定义一个 Agent，把 Model 传进去

好了，现在我们已经有 Model 了。接下来要定义一个 Agent，并且把我们刚才创建的 Model 传给它。

```go
agent := llmagent.New("code-agent",
    llmagent.WithModel(modelInstance),
    llmagent.WithGenerationConfig(model.GenerationConfig{
        Stream:          true,
        ThinkingEnabled: new(os.Getenv("ENABLE_THINKING") == "true"),
    }),
)
```

**这段代码在干嘛？**

我们调用了 `llmagent.New()` 这个函数，创建了一个 Agent。来看看我们传了什么进去：

1. **第一个参数**：`"code-agent"` —— 这是给 Agent 起的名字，随便起，只要你自己能认出来就行。
2. **`llmagent.WithModel(modelInstance)`** —— 这句话的意思是："嘿 Agent，你就用这个 Model 来回答问题吧。"
3. **`llmagent.WithGenerationConfig(...)`** —— 这里我们可以传一些关于模型生成的配置参数，比如：

   - `Stream: true`：开启流式输出。什么意思呢？就是模型生成回答的时候，不是一个字一个字蹦出来给你看的（就像 ChatGPT 那种效果），而不是等全部生成完再一次性显示。
   - `ThinkingEnabled: new(os.Getenv("ENABLE_THINKING") == "true")`：这里我们从环境变量 `ENABLE_THINKING` 读取配置，如果它的值是 `"true"`，就开启思考模式；否则就不开启。

> 💡 **你可能会好奇**：`new(os.Getenv("ENABLE_THINKING") == "true")` 是什么鬼？为什么写得这么复杂？  
> 这是因为 `ThinkingEnabled` 这个参数想要的是一个 `*bool`（bool 型的指针）。我们先通过 `os.Getenv("ENABLE_THINKING") == "true"` 得到一个 `bool` 值（true 或 false），然后用 `new()` 把这个 `bool` 值转换成一个指针。这样就能传进去了。

---

## 第三步：创建一个 Runner，把 Agent 传进去

Agent 定义好了，接下来我们需要一个"运行器"来跑这个 Agent。这就是 `Runner` 的作用。

```go
runner := runner.NewRunner("oh-my-codeagent", agent)
```

**这段代码在干嘛？**

我们调用了 `runner.NewRunner()` 这个函数，创建了一个 Runner。它接收两个参数：

1. **第一个参数**：`"oh-my-codeagent"` —— 这是 Runner 的名字，同样随便起。
2. **第二个参数**：`agent` —— 就是我们刚才定义的那个 Agent。

> 🤔 **你可能会问**：为什么要有 Runner 这个东西？Agent 不能自己跑吗？  
> 其实 Runner 负责的是"运行时"的事情。比如：管理会话、处理事件流、协调多个 Agent 之类的。对于我们这个最小 Demo 来说，你只要知道"Runner 是用来跑 Agent 的"就够了。后面我们会慢慢讲到 Runner 更多的作用。

---

## 第四步：让 Agent 跑起来！

现在万事俱备，我们来让 Agent 跑起来吧！

```go
events, err := runner.Run(context.Background(),
    "user-001",
    "session-001",
    model.NewUserMessage("你好"),
)
if err != nil {
    log.Fatal(err)
}
```

**这段代码在干嘛？**

我们调用了 `runner.Run()` 这个方法，让 Agent 开始工作。它接收几个参数：

1. **`context.Background()`** —— 上下文。
2. **`"user-001"`** —— 这是用户 ID。用来区分"谁在跟 Agent 说话"。
3. **`"session-001"`** —— 这是会话 ID。用来区分"这次对话是哪次"。如果同一个用户聊了两次，每次的 `session-id` 应该不一样，这样 Agent 才知道"这是新的对话"还是"继续上次的对话"。
4. **`model.NewUserMessage("你好")`** —— 这是我们发给 Agent 的消息。这里就是简单地发了个"你好"。

然后，`runner.Run()` 会返回两个东西：`events`（一个"通道"，你可以把它理解成一个"数据流"）和 `err`（如果出错了，这里会有错误信息）。

---

## 第五步：消费这个"数据流"，把 Agent 的回答打印出来

刚才说了，`runner.Run()` 返回了一个 `events` 通道。现在我们需要"消费"这个通道，把 Agent 的回答打印出来。

```go
for event := range events {
    if event != nil && len(event.Choices) > 0 {
        choice := event.Choices[0]
        if choice.Delta.Content != "" {
            fmt.Print(choice.Delta.Content)
        }
    }
}
fmt.Println()
```

**这段代码在干嘛？**

我们在这个循环里消费 `events` 这个"数据流"。每当 Agent 产生一个新的事件（比如生成了一小段文字），我们就把它取出来，然后打印到终端上。

注意这里用的是 `fmt.Print()`（没有 `ln`），意思是"不换行打印"。因为我们用的是流式输出，模型是一个字一个字生成的，所以我们得一个字一个字地打印，这样看起来就像是模型在"边想边说"。等所有事件都处理完了，我们再打印一个换行符（`fmt.Println()`），让光标移到下一行。

---

## 插播：这个选项模式是什么鬼？

如果你仔细看我们刚才的代码，可能会发现一个特点：我们在创建 Model、创建 Agent 的时候，用的都是类似这样的写法：

```go
openai.New("model-id",
    openai.WithAPIKey("key"),
    openai.WithBaseURL("url"),
)
```

这种写法，在 Go 语言里叫做**选项模式**（Options Pattern）。它的作用是：让你可以**选择性地**传参数。

什么意思呢？比如说，如果你只想传 `APIKey`，不想传 `BaseURL`，那你可以只写一行：

```go
openai.New("model-id",
    openai.WithAPIKey("key"),
)
```

这在 Python 里就像是"可选参数"：

```python
def openai_new(model_id, api_key=None, base_url=None):
    pass

# 只传 api_key
openai_new("model-id", api_key="key")
```

所以选项模式的好处就是：参数可选，想传就传，不想传就拉倒。代码看起来也更清晰。

---

## 运行一下，看看效果！

好了，代码讲完了。我们来运行一下，看看效果：

```bash
go run main.go
```

如果一切正常，你应该会看到类似这样的输出（因为是流式输出，所以字是一个一个蹦出来的）：

```
你好！我是 code-agent，一个基于 tRPC-Agent-Go 框架的代码助手。有什么可以帮你的吗？
```

---

## 总结一下

我们刚才写了个最小 Demo，用到了 tRPC-Agent-Go 框架。回顾一下，要跑起一个 Agent，我们需要：

1. **定义 Model**：告诉 Agent 用哪个大模型。
2. **定义 Agent**：把 Model 传进去，还可以设置一些生成参数（比如是否流式输出）。
3. **定义 Runner**：把 Agent 传进去，让它负责"运行时"的事情。
4. **调用 Runner.Run()**：传入用户 ID、会话 ID、用户消息，然后得到一个"数据流"。
5. **消费这个"数据流"**：把 Agent 的回答打印出来。

是不是还挺简单的？当然，这个 Demo 还很粗糙，后面我们会慢慢给它加功能，让它变得更像一个"代码助手"。

下一节，我们会教 Agent 怎么用工具（Tool），这样它就能帮我们做实际的事情了！
