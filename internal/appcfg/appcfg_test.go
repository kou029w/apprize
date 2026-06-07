package appcfg

import (
	"strings"
	"testing"
)

func TestParseText(t *testing.T) {
	cfg := "alerts=gotify://host/token1\nteam ops=discord://id/token2\n# comment\n"
	got := ParseText(cfg)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].URL != "gotify://host/token1" {
		t.Fatalf("first URL = %q", got[0].URL)
	}
	if len(got[0].Tags) != 1 || got[0].Tags[0] != "alerts" {
		t.Fatalf("first tags = %v", got[0].Tags)
	}
	if len(got[1].Tags) != 2 {
		t.Fatalf("second tags = %v", got[1].Tags)
	}
}

func TestParseYAML(t *testing.T) {
	cfg := `
version: 1
urls:
  - gotify://host/token
  - discord://id/token:
      tag: alerts,team
`
	got, err := ParseYAML(cfg)
	if err != nil {
		t.Fatalf("ParseYAML error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].URL != "discord://id/token" {
		t.Fatalf("second URL = %q", got[1].URL)
	}
	if len(got[1].Tags) != 2 {
		t.Fatalf("second tags = %v", got[1].Tags)
	}
}

func TestTagMatching(t *testing.T) {
	groups := ParseTagExpr("devops alerts, finance")
	if !Match(groups, []string{"devops", "alerts"}) {
		t.Fatal("expected devops+alerts to match")
	}
	if !Match(groups, []string{"finance"}) {
		t.Fatal("expected finance to match")
	}
	if Match(groups, []string{"devops"}) {
		t.Fatal("expected devops alone not to match")
	}
}

func TestMaskURL(t *testing.T) {
	masked := MaskURL("gotify://host/SECRETTOKEN?apikey=ABC")
	if strings.Contains(masked, "SECRETTOKEN") || strings.Contains(masked, "ABC") {
		t.Fatalf("secret leaked in %q", masked)
	}
}

func TestURLIdentityAndService(t *testing.T) {
	if URLID("gotify://host/token") == "" {
		t.Fatal("expected non-empty URLID")
	}
	if ServiceName("tgram://token/chat") != "Telegram" {
		t.Fatal("expected Telegram service mapping")
	}
	if Scheme("discord://id/token") != "discord" {
		t.Fatal("expected discord scheme")
	}
}
