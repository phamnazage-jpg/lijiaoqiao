package sms

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type smsExampleRoot struct {
	SMS Config `yaml:"sms"`
}

func TestSMSExampleYAML_MatchesRuntimeConfigShape(t *testing.T) {
	path := filepath.Join("..", "..", "config", "sms.example.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read sms example config: %v", err)
	}

	var root smsExampleRoot
	if err := yaml.Unmarshal(content, &root); err != nil {
		t.Fatalf("failed to parse sms example config: %v", err)
	}

	if root.SMS.Provider != ProviderTencent {
		t.Fatalf("expected provider %q, got %q", ProviderTencent, root.SMS.Provider)
	}
	if root.SMS.CodeLength != 6 {
		t.Fatalf("expected code length 6, got %d", root.SMS.CodeLength)
	}
	if root.SMS.CodeExpireMins != 5 {
		t.Fatalf("expected code expiry 5, got %d", root.SMS.CodeExpireMins)
	}
	if root.SMS.SignName == "" {
		t.Fatal("expected sign_name to be populated in example config")
	}
	if root.SMS.TemplateCode == "" {
		t.Fatal("expected template_code to be populated in example config")
	}
	if root.SMS.Region == "" {
		t.Fatal("expected region to be populated in example config")
	}
}
