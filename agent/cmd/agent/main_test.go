package main

import (
	"strings"
	"testing"
)

func TestAgentWSURL(t *testing.T) {
	got, err := agentWSURL("http://localhost:8080", "token-1")
	if err != nil {
		t.Fatalf("agentWSURL: %v", err)
	}
	if got != "ws://localhost:8080/api/agent/ws?token=token-1" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestBuildRegisterUsesProfile(t *testing.T) {
	reg := buildRegister(config{
		AgentID:     "agent-1",
		Name:        "Lab PC 01",
		GroupName:   "classroom-a",
		MockProfile: "windows-lab",
	})
	if reg.OS != "Windows 11 Pro" || reg.MemoryMB != 16384 {
		t.Fatalf("unexpected register payload: %+v", reg)
	}
	if !strings.Contains(reg.Hostname, ".win.lab") {
		t.Fatalf("expected windows hostname suffix: %s", reg.Hostname)
	}
}

func TestExecuteCommand(t *testing.T) {
	stdout, stderr, exitCode := executeCommand("echo labops")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "labops") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}
