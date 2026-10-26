package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const systemPrompt = `你是 rsWGCNA 共表达分析工具的 AI 助手，请用中文回答。
你了解 WGCNA 原理、本工具的 7 步操作流程和参数含义，并且可以调用工具读取用户当前
分析的实际参数和结果（每一步的参数和结果都可通过工具读取），也可以检索文献。

回答必须遵守：
1. 涉及用户的分析结果/参数时，先调用相应工具读取真实数据，再回答；不要凭空猜测。
2. 每个事实性结论都要标注来源：
   - 知识库 → [来源: wgcna-principles / pipeline / params / troubleshooting]
   - 工具读取的结果 → [来源: params.json / step2_sft.json / step3_modules.json 等]
   - 文献 → [PMID: xxx] 或 [DOI: xxx]，并给出链接。
3. 文献只能引用 search_literature 工具实际返回的文献；严禁编造 PMID、DOI、作者、
   期刊或年份。如果某篇文献不在工具返回结果里，不要引用；不确定就明确说"未检索到"。
4. 当回答涉及生物学机制、通路、基因功能、研究方法或研究背景时，主动调用
   search_literature 检索文献支撑你的说法，不要等用户明确要求才检索。
5. 无法从工具或知识库得到的信息，要明确说明这是基于你的通用知识，而非用户数据。
6. 文献引用格式：正文中用「第一作者 et al. (年份)」标注（如 Charng et al., 2007），
   并在回答末尾单独列出 "References" 小节，逐条给出完整引用（作者、标题、期刊、
   年份、PMID/DOI 链接），正文中的标注与末尾 References 一一对应。

下面是检索到的相关知识片段（按相关性排序），请优先依据它们回答：`

// ToolExecutor executes a named tool with JSON-decoded arguments and returns a
// plain-text result. It is implemented by the api package (job-dir access).
type ToolExecutor interface {
	Execute(name string, args map[string]any) (string, error)
}

// Navigator assembles context (system prompt + retrieved knowledge + tools) and
// calls the LLM, optionally in an agent loop that invokes tools.
type Navigator struct {
	client *Client
	kb     *KB
	tools  []Tool
	exec   ToolExecutor
}

// NewNavigator builds a Navigator from the persisted config.
func NewNavigator(store *Store) (*Navigator, error) {
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	key, err := store.APIKey(cfg)
	if err != nil {
		return nil, err
	}
	return &Navigator{
		client: &Client{BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: key},
		kb:     NewKB(KnowledgeDocs(), 800),
	}, nil
}

// SetTools attaches tool definitions + an executor (enables the agent loop).
func (n *Navigator) SetTools(tools []Tool, exec ToolExecutor) {
	n.tools = tools
	n.exec = exec
}

// Configured reports whether an LLM provider + key are set.
func (n *Navigator) Configured() bool {
	return n.client.BaseURL != "" && n.client.APIKey != ""
}

// Answer returns the reply for a question (optionally with error context).
func (n *Navigator) Answer(question, errorCtx string) (string, error) {
	if !n.Configured() {
		return "", fmt.Errorf("AI 领航员未配置：请先在设置里填入 LLM 厂商的 API key")
	}
	return n.client.Chat(n.buildMessages(question, errorCtx))
}

// AnswerStream streams the reply, invoking onDelta per chunk.
func (n *Navigator) AnswerStream(question, errorCtx string, onDelta func(string)) error {
	if !n.Configured() {
		return fmt.Errorf("AI 领航员未配置：请先在设置里填入 LLM 厂商的 API key")
	}
	return n.client.ChatStream(n.buildMessages(question, errorCtx), onDelta)
}

// AnswerAgent runs the tool-calling agent loop and returns the final answer.
func (n *Navigator) AnswerAgent(question, errorCtx string) (string, error) {
	if !n.Configured() {
		return "", fmt.Errorf("AI 领航员未配置：请先在设置里填入 LLM 厂商的 API key")
	}
	if n.exec == nil || len(n.tools) == 0 {
		return n.Answer(question, errorCtx)
	}
	return n.runAgent(n.buildMessages(question, errorCtx))
}

func (n *Navigator) runAgent(msgs []Message) (string, error) {
	const maxRounds = 8
	for round := 0; round < maxRounds; round++ {
		resp, err := n.client.ChatFull(msgs, n.tools)
		if err != nil {
			return "", err
		}
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}
		msgs = append(msgs, resp)
		for _, tc := range resp.ToolCalls {
			args := map[string]any{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			result, err := n.exec.Execute(tc.Function.Name, args)
			if err != nil {
				result = "工具执行出错: " + err.Error()
			}
			msgs = append(msgs, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return "", fmt.Errorf("AI 助手调用了过多工具，请简化问题")
}

func (n *Navigator) buildMessages(question, errorCtx string) []Message {
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	for _, c := range n.kb.Search(question, 4) {
		sb.WriteString("\n\n--- 知识片段（" + c.Source + "）---\n")
		sb.WriteString(strings.TrimSpace(c.Text))
	}

	var user strings.Builder
	if errorCtx != "" {
		user.WriteString("用户遇到以下报错，请分析原因并给出修改建议：\n")
		user.WriteString("```\n" + errorCtx + "\n```\n\n")
	}
	user.WriteString("用户提问：")
	user.WriteString(question)

	return []Message{
		{Role: "system", Content: sb.String()},
		{Role: "user", Content: user.String()},
	}
}
