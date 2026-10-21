package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAndNotifyNewVersion(t *testing.T) {
	tempDir := t.TempDir()
	versionFile := filepath.Join(tempDir, "data", "version.txt")

	var notified []matchNotification
	mockNotify := func(msg matchNotification) {
		notified = append(notified, msg)
	}

	// 1. First run: file does not exist -> must notify and save version
	firstRun := checkAndNotifyNewVersion(mockNotify, versionFile)
	if !firstRun {
		t.Fatalf("expected firstRun to return true")
	}
	if len(notified) != 1 {
		t.Fatalf("expected 1 notification on first run, got %d", len(notified))
	}
	if !strings.Contains(notified[0].Text, AppVersion) {
		t.Fatalf("expected notification to mention %s, got: %s", AppVersion, notified[0].Text)
	}
	if !strings.Contains(notified[0].Text, "Что нового") {
		t.Fatalf("expected changelog to mention 'Что нового', got: %s", notified[0].Text)
	}

	// Verify file was written
	content, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatalf("failed to read saved version file: %v", err)
	}
	if strings.TrimSpace(string(content)) != AppVersion {
		t.Fatalf("saved version %q != expected %q", string(content), AppVersion)
	}

	// 2. Second run: same version -> must NOT notify
	notified = nil
	secondRun := checkAndNotifyNewVersion(mockNotify, versionFile)
	if secondRun {
		t.Fatalf("expected secondRun to return false")
	}
	if len(notified) != 0 {
		t.Fatalf("expected 0 notifications on second run, got %d", len(notified))
	}

	// 3. Third run: previous version was older -> must notify and update file
	if err := os.WriteFile(versionFile, []byte("v1.0.0"), 0644); err != nil {
		t.Fatalf("failed to write old version: %v", err)
	}
	notified = nil
	upgradeRun := checkAndNotifyNewVersion(mockNotify, versionFile)
	if !upgradeRun {
		t.Fatalf("expected upgradeRun to return true")
	}
	if len(notified) != 1 {
		t.Fatalf("expected 1 notification on upgrade, got %d", len(notified))
	}
	updated, _ := os.ReadFile(versionFile)
	if strings.TrimSpace(string(updated)) != AppVersion {
		t.Fatalf("expected file updated to %q, got %q", AppVersion, string(updated))
	}
}
