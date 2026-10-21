package ai

import (
	"fmt"
	"strings"
)

const systemPrompt = `你是 WGCNA 共表达分析工具的"AI 领航员"。请用中文回答。
你了解 WGCNA 原理、本工具的 7 步操作流程和参数含义。回答要简洁、准确，引导用户
操作；如果用户报错，结合错误信息定位原因并给出可操作的修改建议。
下面是检索到的相关知识片段（按相关性排序），请优先依据它们回答：`

// Navigator assembles context (system prompt + retrieved knowledge + optional
// error context) and calls the LLM.
type Navigator struct {
	client *Client
	kb     *KB
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
