package memory

type KnowledgeStore struct {
	answers map[string]string
}

func NewKnowledgeStore() *KnowledgeStore {
	return &KnowledgeStore{answers: map[string]string{
		"quota":   "当前版本暂未接入实时配额查询，建议先在控制台查看配额页；如需人工协助请回复人工客服。",
		"token":   "当前版本暂未接入实时 Token 统计，建议先查看控制台用量页；如需人工协助请回复人工客服。",
		"error":   "若您遇到错误，请提供报错时间、请求 ID 和复现步骤，我们会优先协助排查。",
		"general": "已收到您的问题。当前系统可处理常见 FAQ；若问题复杂或涉及账户安全，会自动转人工。",
	}}
}

func (s *KnowledgeStore) Answer(intent string) string {
	if answer, ok := s.answers[intent]; ok {
		return answer
	}
	return s.answers["general"]
}
