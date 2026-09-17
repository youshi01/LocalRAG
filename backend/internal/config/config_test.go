package config

import (
	"path/filepath"
	"testing"
)

func TestLoadServerConfigDerivesStagingDirectoryFromUploadDirectory(t *testing.T) {
	t.Setenv("UPLOAD_DIR", "/srv/localrag/uploads")
	t.Setenv("STAGING_DIR", "")

	serverConfig := LoadServerConfig()
	if want := filepath.Join("/srv/localrag", "staging"); serverConfig.StagingDir != want {
		t.Fatalf("expected staging directory %q, got %q", want, serverConfig.StagingDir)
	}
}

func TestLoadServerConfigUsesExplicitStagingDirectory(t *testing.T) {
	t.Setenv("UPLOAD_DIR", "/srv/localrag/uploads")
	t.Setenv("STAGING_DIR", "/mnt/localrag/staging")

	serverConfig := LoadServerConfig()
	if serverConfig.StagingDir != "/mnt/localrag/staging" {
		t.Fatalf("expected explicit staging directory, got %q", serverConfig.StagingDir)
	}
}
