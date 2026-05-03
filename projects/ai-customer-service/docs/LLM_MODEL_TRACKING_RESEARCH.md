# LLM 大模型信息追踪工具调研报告

> 调研时间：2026-05-03
> 调研目的：为「立交桥」项目收集大模型提供商、版本、运营商、免费/收费政策等决策信息

---

## 一、现有追踪工具对比

### 1.1 GitHub 项目类

| 项目 | 语言 | ⭐ | 核心功能 | 适用场景 |
|------|------|---|---------|----------|
| **AIPriceIndex** | TypeScript | 1 | 爬取 LLM 定价页面，对比历史价格 | 价格监控 |
| **llm-price-tracker** | Python | 0 | 12 家提供商 35+ 模型价格计算器 | 成本计算 |
| **llm-pricing-index** | None | 0 | 20+ 模型月度定价（自动更新） | 定期对账 |
| **llm-cost-tracker** | TypeScript | 0 | 跨提供商使用追踪 + 预算告警 | 个人使用追踪 |
| **ohmytoken** | TypeScript | 0 | 23+ LLM 实时价格 MCP Server | Agent 内嵌 |
| **Model-ID-Cheatsheet** | Go | - | 107 模型 19 提供商 ID/定价/上下文窗口 | AI Coding 精确模型 ID |
| **truefoundry/models** | YAML | - | 21 提供商 1000+ 模型配置（定价/特性/限制） | 综合参考 |
| **FreeRide** | Python | 170 | OpenRouter 免费模型自动路由 + 熔断 | 免费 AI |

### 1.2 综合信息平台

| 平台 | URL | 核心功能 |
|------|-----|---------|
| **ClawHub** | clawhub.ai | OpenClaw 技能市场，52.7k tools，热门 Skill 排行 |
| **OpenRouter** | openrouter.ai/models | 724 模型统一 API，支持免费模型排名 |
| **MCP Registry** | registry.modelcontextprotocol.io | Model Context Protocol 服务器目录 |
| **HuggingFace** | huggingface.co/models | 开源模型大全（下载量/评测/GGUF） |

---

## 二、大模型信息追踪网站/项目详细整理

### 2.1 免费综合型

#### OpenRouter（推荐）
- **网址**：openrouter.ai/models
- **覆盖**：724 模型，OpenAI/Anthropic/Google/DeepSeek/Meta 等
- **核心信息**：
  - 每个模型：输入/输出价格（$/M tokens）
  - 上下文窗口、Capabilities（Vision/Tools/JSON mode）
  - 免费模型标记（`:free` 后缀）
  - 模型质量排名（ELO score）
- **免费政策**：30+ 免费模型，带智能熔断
- **适用**：项目选型首选参考

#### FreeRide（专为 OpenClaw）
- **网址**：clawhub.ai/skills/free-ride（GitHub: Shaivpidi/FreeRide）
- **功能**：自动管理 OpenRouter 免费模型，按质量排名 + 自动切换
- **使用方式**：`freeride auto` 一键配置，watcher 守护自动熔断
- **安装**：`npx clawhub@latest install free-ride`

### 2.2 模型精确 ID 参考

#### Model-ID-Cheatsheet（⭐推荐 AI Coding）
- **GitHub**：aezizhu/Model-ID-Cheatsheet
- **功能**：Stop AI coding agents from hallucinating outdated model names
- **规模**：107 models × 19 providers，Go 编写，零外部调用，sub-ms 响应
- **使用**：MCP Server (`model-id-cheatsheet`)，Claude Code/Cursor/Windsurf 均可接入
- **工具**：get_model_info / list_models / recommend_model / check_model_status / compare_models / search_models

### 2.3 社区维护型模型注册表

#### truefoundry/models（最全面）
- **GitHub**：truefoundry/models
- **覆盖**：21 provider，1000+ 模型配置文件
- **信息维度**：model ID / pricing / context window / features / modalities / limits
- **更新**：社区驱动，实时更新

#### Universal Model Registry (UMR)
- **GitHub**：EvanZhouDev/umr
- **功能**：本地模型统一注册表，支持 LM Studio/Ollama/Jan
- **适用**：本地部署场景

### 2.4 MCP 协议生态

#### MCP Registry
- **网址**：registry.modelcontextprotocol.io
- **说明**：Model Context Protocol 官方服务器目录
- **热门服务器**：
  - `inference.sh`：150+ AI 应用（图像/视频/音频/LLM/3D）
  - `trading`：AI 交易策略回测
  - `google-ads`：Google Ads 管理
  - `AgentDM`：跨模型 Agent 通讯

---

## 三、大模型提供商免费政策汇总

| 提供商 | 免费模型 | 免费额度 | 限制 |
|--------|----------|----------|------|
| **OpenRouter** | 30+ | 每日一定量 | 需 API key，部分模型限流 |
| **Google** | Gemini Flash 2.0 | 一定量/分钟 | 需要信用卡验证 |
| **DeepSeek** | V3 / Coder V3 | 大量 | 国内访问可能受限 |
| **Groq** | Llama/Mixtral | 高速免费 | 仅限特定模型 |
| **Anthropic** | Claude 3.5 Haiku | 有限 | 需订阅或有额度 |
| **Cohere** | Command R+ | 一定量 | 需注册 |

---

## 四、项目选型建议

| 需求场景 | 推荐工具 | 原因 |
|----------|----------|------|
| **项目模型决策** | OpenRouter 模型页 + truefoundry/models | 覆盖全、价格清、能力明 |
| **Coding Agent 用模型 ID** | Model-ID-Cheatsheet MCP | 精确、无幻觉、自动更新 |
| **想用免费 AI** | FreeRide + OpenRouter | 一键配置、自动熔断、零成本 |
| **追踪使用成本** | llm-cost-tracker / AIPriceIndex | 预算告警、历史对比 |
| **构建 Agent 工具链** | MCP Registry + ClawHub Skills | 60+ MCP 服务器 + 52k Skills |

---

## 五、「立交桥」项目落地建议

**需求**：了解最新大模型信息（提供商/版本/运营商/免费政策/收费）

### 推荐方案：OpenRouter 为核心 + 本地模型数据库

1. **日常参考**：直接使用 openrouter.ai/models 查看所有模型定价和能力
2. **自动化获取**：集成 truefoundry/models（YAML 配置，版本化可维护）
3. **本地知识库**：如需离线查询，部署 Model-ID-Cheatsheet MCP Server
4. **开发环境**：FreeRide 让开发测试零成本

### 如需自建追踪系统

**数据源**：
- OpenRouter API → 每日同步模型列表 + 定价
- Anthropic/Google 官方定价页 → 正则解析
- GitHub trending → 新模型发布动态

**技术方案**：Python 爬虫 + SQLite 本地存储 + Web 界面
参考项目：AIPriceIndex / llm-price-tracker

---

