package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseKiroInput_SingleObject(t *testing.T) {
	input := `{"refreshToken":"rt-123","clientId":"cid","clientSecret":"cs","region":"us-east-1","startUrl":"https://example.awsapps.com/start","provider":"idc","machineId":"m1","email":"user@example.com"}`

	creds, err := ParseKiroInput([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].RefreshToken != "rt-123" {
		t.Errorf("RefreshToken = %q, want %q", creds[0].RefreshToken, "rt-123")
	}
	if creds[0].Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", creds[0].Email, "user@example.com")
	}
}

func TestParseKiroInput_Array(t *testing.T) {
	input := `[
		{"refreshToken":"rt-1","clientId":"c1","clientSecret":"s1","region":"us-east-1","startUrl":"","provider":"idc","machineId":"","email":"a@b.com"},
		{"refreshToken":"rt-2","clientId":"c2","clientSecret":"s2","region":"eu-west-1","startUrl":"","provider":"idc","machineId":"","email":"c@d.com"}
	]`

	creds, err := ParseKiroInput([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}
	if creds[1].Region != "eu-west-1" {
		t.Errorf("creds[1].Region = %q, want %q", creds[1].Region, "eu-west-1")
	}
}

func TestParseKiroInput_Empty(t *testing.T) {
	_, err := ParseKiroInput([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseKiroInput_Whitespace(t *testing.T) {
	_, err := ParseKiroInput([]byte("   \n  "))
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

func TestParseKiroInput_InvalidJSON(t *testing.T) {
	_, err := ParseKiroInput([]byte("{not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestConvertKiroCredential(t *testing.T) {
	ext := KiroExternalCredential{
		RefreshToken: "rt-abc",
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		Region:       "us-west-2",
		StartURL:     "https://start.example.com/start",
		Provider:     "idc",
		MachineID:    "machine-1",
		Email:        "test@example.com",
	}

	got := ConvertKiroCredential(ext)

	if got.Type != "kiro" {
		t.Errorf("Type = %q, want %q", got.Type, "kiro")
	}
	if got.AccessToken != "" {
		t.Errorf("AccessToken = %q, want empty", got.AccessToken)
	}
	if got.RefreshToken != "rt-abc" {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "rt-abc")
	}
	if got.ExpiresAt != "2020-01-01T00:00:00Z" {
		t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, "2020-01-01T00:00:00Z")
	}
	if got.AuthMethod != "idc" {
		t.Errorf("AuthMethod = %q, want %q", got.AuthMethod, "idc")
	}
	if got.ClientID != "my-client" {
		t.Errorf("ClientID = %q, want %q", got.ClientID, "my-client")
	}
	if got.ClientSecret != "my-secret" {
		t.Errorf("ClientSecret = %q, want %q", got.ClientSecret, "my-secret")
	}
	if got.StartURL != "https://start.example.com/start" {
		t.Errorf("StartURL = %q, want %q", got.StartURL, "https://start.example.com/start")
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "test@example.com")
	}
}

func TestGenerateKiroFileName_WithEmail(t *testing.T) {
	cred := KiroExternalCredential{
		Provider: "idc",
		Email:    "user@example.com",
	}
	got := GenerateKiroFileName(cred, 0)
	want := "kiro-idc-user-example.com.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateKiroFileName_WithStartURL(t *testing.T) {
	cred := KiroExternalCredential{
		Provider: "idc",
		StartURL: "https://myorg.awsapps.com/start",
	}
	got := GenerateKiroFileName(cred, 0)
	want := "kiro-idc-myorg.awsapps.com.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateKiroFileName_Fallback(t *testing.T) {
	cred := KiroExternalCredential{
		Provider: "idc",
	}
	got := GenerateKiroFileName(cred, 3)
	want := "kiro-idc-3.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateKiroFileName_EmptyProvider(t *testing.T) {
	cred := KiroExternalCredential{
		Email: "user@test.com",
	}
	got := GenerateKiroFileName(cred, 0)
	want := "kiro-unknown-user-test.com.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateKiroFileName_EmailPriorityOverStartURL(t *testing.T) {
	cred := KiroExternalCredential{
		Provider: "idc",
		Email:    "user@example.com",
		StartURL: "https://other.awsapps.com/start",
	}
	got := GenerateKiroFileName(cred, 0)
	want := "kiro-idc-user-example.com.json"
	if got != want {
		t.Errorf("email should take priority: got %q, want %q", got, want)
	}
}

// --- ReadKiroSSOFiles tests ---

func writeTestJSON(t *testing.T, dir, name string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadKiroSSOFiles_WithAutoDiscovery(t *testing.T) {
	dir := t.TempDir()

	tokenPath := writeTestJSON(t, dir, "kiro-auth-token.json", KiroSSOTokenFile{
		AccessToken:  "at-123",
		RefreshToken: "rt-456",
		ExpiresAt:    "2026-01-01T00:00:00Z",
		AuthMethod:   "IdC",
		Provider:     "Enterprise",
		Region:       "ap-northeast-1",
		ClientIDHash: "abc123hash",
	})
	writeTestJSON(t, dir, "abc123hash.json", KiroSSOClientFile{
		ClientID:     "my-client-id",
		ClientSecret: "my-client-secret",
	})

	cred, err := ReadKiroSSOFiles(tokenPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.Type != "kiro" {
		t.Errorf("Type = %q, want %q", cred.Type, "kiro")
	}
	if cred.AccessToken != "at-123" {
		t.Errorf("AccessToken = %q, want %q", cred.AccessToken, "at-123")
	}
	if cred.RefreshToken != "rt-456" {
		t.Errorf("RefreshToken = %q, want %q", cred.RefreshToken, "rt-456")
	}
	if cred.AuthMethod != "IdC" {
		t.Errorf("AuthMethod = %q, want %q", cred.AuthMethod, "IdC")
	}
	if cred.ClientID != "my-client-id" {
		t.Errorf("ClientID = %q, want %q", cred.ClientID, "my-client-id")
	}
	if cred.ClientSecret != "my-client-secret" {
		t.Errorf("ClientSecret = %q, want %q", cred.ClientSecret, "my-client-secret")
	}
	if cred.ClientIDHash != "abc123hash" {
		t.Errorf("ClientIDHash = %q, want %q", cred.ClientIDHash, "abc123hash")
	}
}

func TestReadKiroSSOFiles_ExplicitClientFile(t *testing.T) {
	dir := t.TempDir()

	tokenPath := writeTestJSON(t, dir, "token.json", KiroSSOTokenFile{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    "2026-01-01T00:00:00Z",
		AuthMethod:   "builder-id",
		Provider:     "AWS",
	})
	clientPath := writeTestJSON(t, dir, "custom-client.json", KiroSSOClientFile{
		ClientID:     "explicit-cid",
		ClientSecret: "explicit-cs",
	})

	cred, err := ReadKiroSSOFiles(tokenPath, clientPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.ClientID != "explicit-cid" {
		t.Errorf("ClientID = %q, want %q", cred.ClientID, "explicit-cid")
	}
	if cred.ClientSecret != "explicit-cs" {
		t.Errorf("ClientSecret = %q, want %q", cred.ClientSecret, "explicit-cs")
	}
}

func TestReadKiroSSOFiles_MissingClientFileWarns(t *testing.T) {
	dir := t.TempDir()

	tokenPath := writeTestJSON(t, dir, "token.json", KiroSSOTokenFile{
		RefreshToken: "rt",
		ExpiresAt:    "2026-01-01T00:00:00Z",
		AuthMethod:   "IdC",
		Provider:     "Enterprise",
		ClientIDHash: "nonexistent",
	})

	// Should succeed but without clientId/clientSecret
	cred, err := ReadKiroSSOFiles(tokenPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.ClientID != "" {
		t.Errorf("ClientID should be empty, got %q", cred.ClientID)
	}
}

func TestReadKiroSSOFiles_MissingRefreshToken(t *testing.T) {
	dir := t.TempDir()

	tokenPath := writeTestJSON(t, dir, "token.json", KiroSSOTokenFile{
		AccessToken: "at",
		ExpiresAt:   "2026-01-01T00:00:00Z",
	})

	_, err := ReadKiroSSOFiles(tokenPath, "")
	if err == nil {
		t.Fatal("expected error for missing refreshToken")
	}
}

func TestReadKiroSSOFiles_TokenWithInlineCredentials(t *testing.T) {
	dir := t.TempDir()

	// Token file already has clientId/clientSecret inline (no clientIdHash lookup needed)
	tokenPath := writeTestJSON(t, dir, "token.json", KiroSSOTokenFile{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    "2026-01-01T00:00:00Z",
		AuthMethod:   "idc",
		Provider:     "Enterprise",
		ClientID:     "inline-cid",
		ClientSecret: "inline-cs",
	})

	cred, err := ReadKiroSSOFiles(tokenPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.ClientID != "inline-cid" {
		t.Errorf("ClientID = %q, want %q", cred.ClientID, "inline-cid")
	}
}

func TestResolvePath_Relative(t *testing.T) {
	got := ResolvePath("some/relative/path.json")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestResolvePath_Absolute(t *testing.T) {
	got := ResolvePath("/absolute/path.json")
	if got != "/absolute/path.json" {
		t.Errorf("got %q, want %q", got, "/absolute/path.json")
	}
}

func TestResolvePath_Tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := ResolvePath("~/test.json")
	want := filepath.Join(home, "test.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
