package token_test

import "testing"

type auditTemplateCase struct {
	ID         string
	Name       string
	TriggerCase string
	Assertions []string
}

func TestTokenAuditTemplateCases(t *testing.T) {
	t.Parallel()

	cases := []auditTemplateCase{
		{
			ID:          "TOK-AUD-001",
			Name:        "签发成功事件",
			TriggerCase: "TOK-LIFE-001",
			Assertions: []string{
				"存在 token.issue.success",
				"event_id/request_id/result_code/route/created_at 齐全",
			},
		},
		{
			ID:          "TOK-AUD-002",
			Name:        "签发失败事件",
			TriggerCase: "TOK-LIFE-002",
			Assertions: []string{
				"存在 token.issue.fail",
				"result_code 准确",
			},
		},
		{
			ID:          "TOK-AUD-003",
			Name:        "鉴权失败事件",
			TriggerCase: "无效 token 访问受保护接口",
			Assertions: []string{
				"存在 token.authn.fail",
				"包含 request_id",
			},
		},
		{
			ID:          "TOK-AUD-004",
			Name:        "越权事件",
			TriggerCase: "TOK-LIFE-008",
			Assertions: []string{
				"存在 token.authz.denied",
				"包含 subject_id",
			},
		},
		{
			ID:          "TOK-AUD-005",
			Name:        "吊销事件",
			TriggerCase: "TOK-LIFE-005",
			Assertions: []string{
				"存在 token.revoke.success",
				"包含 token_id",
			},
		},
		{
			ID:          "TOK-AUD-006",
			Name:        "query key 拒绝事件",
			TriggerCase: "query key 访问受保护接口",
			Assertions: []string{
				"存在 token.query_key.rejected",
				"不含敏感值",
			},
		},
		{
			ID:          "TOK-AUD-007",
			Name:        "事件不可篡改",
			TriggerCase: "重复读取同 event_id",
			Assertions: []string{
				"核心字段不可变",
				"时间顺序正确",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ID, func(t *testing.T) {
			t.Skipf("模板用例，待接入实现: %s", tc.Name)
		})
	}
}
