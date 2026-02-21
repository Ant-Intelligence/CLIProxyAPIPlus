package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		ClientID: "f_R3uaW3B0d6cor4",
	}
	got := GenerateKiroFileName(cred, 0)
	want := "kiro-idc-myorg.awsapps.com-f-r3uaw3.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateKiroFileName_StartURLWithoutClientID(t *testing.T) {
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

func TestGenerateKiroFileName_SameStartURLDifferentClientID(t *testing.T) {
	cred1 := KiroExternalCredential{
		Provider: "Enterprise",
		StartURL: "https://d-9066017a60.awsapps.com/start/",
		ClientID: "f_R3uaW3B0d6cor4MeoTFXVzLWVhc3QtMQ",
	}
	cred2 := KiroExternalCredential{
		Provider: "Enterprise",
		StartURL: "https://d-9066017a60.awsapps.com/start/",
		ClientID: "g_X5abCD7E8f9ghi",
	}
	name1 := GenerateKiroFileName(cred1, 0)
	name2 := GenerateKiroFileName(cred2, 1)
	if name1 == name2 {
		t.Errorf("filenames should differ for different clientIDs under same startUrl, both got %q", name1)
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

// --- Integration tests: credential validation and diagnosis ---

// TestIntegration_RawKiroFileIsInvalidForServer verifies that a raw Kiro Account Manager
// export (JSON array with camelCase fields) cannot be directly used by the server's
// auth system — it must go through the conversion pipeline first.
func TestIntegration_RawKiroFileIsInvalidForServer(t *testing.T) {
	// This is the exact format exported by Kiro Account Manager
	rawFile := `[
  {
    "refreshToken": "aorAAAAA_fake_refresh_token",
    "clientId": "JnqRJ-wsYBSc6aWgXXXXXLW5vcnRoLTE",
    "clientSecret": "eyJraWQiOiJrZXktMTU4ODcwMjU2NSIsImFsZyI6IkhTMzg0In0.fake",
    "region": "us-east-1",
    "startUrl": "https://d-c367640ad3.awsapps.com/start/",
    "provider": "Enterprise",
    "machineId": ""
  }
]`

	// The server's registerAuthFromFile does json.Unmarshal(data, &metadata)
	// where metadata is map[string]any — a JSON array will fail here.
	var metadata map[string]any
	err := json.Unmarshal([]byte(rawFile), &metadata)
	if err == nil {
		t.Fatal("expected JSON array to fail unmarshalling into map[string]any, but it succeeded")
	}

	// Verify the specific issue: JSON array cannot be unmarshalled into a map
	if !strings.Contains(err.Error(), "cannot unmarshal array") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestIntegration_CamelCaseFieldsNotRecognized verifies that even if a single object
// (non-array) is uploaded directly, camelCase fields won't be found by the server.
func TestIntegration_CamelCaseFieldsNotRecognized(t *testing.T) {
	// Single object in camelCase (user might extract from array manually)
	camelCaseJSON := `{
    "refreshToken": "rt-fake",
    "clientId": "cid-fake",
    "clientSecret": "cs-fake",
    "region": "us-east-1",
    "startUrl": "https://example.awsapps.com/start/",
    "provider": "Enterprise"
  }`

	var metadata map[string]any
	if err := json.Unmarshal([]byte(camelCaseJSON), &metadata); err != nil {
		t.Fatalf("unmarshal should succeed for single object: %v", err)
	}

	// Server checks these snake_case keys — all will be missing/empty
	accessToken, _ := metadata["access_token"].(string)
	if accessToken != "" {
		t.Error("access_token should be empty in camelCase format")
	}

	refreshToken, _ := metadata["refresh_token"].(string)
	if refreshToken != "" {
		t.Error("refresh_token should be empty in camelCase format")
	}

	clientID, _ := metadata["client_id"].(string)
	if clientID != "" {
		t.Error("client_id should be empty in camelCase format")
	}

	providerType, _ := metadata["type"].(string)
	if providerType != "" {
		t.Error("type should be empty — raw Kiro export uses 'provider' not 'type'")
	}

	// The camelCase keys DO exist but won't be found by the server
	if _, ok := metadata["refreshToken"]; !ok {
		t.Error("camelCase 'refreshToken' should exist in raw format")
	}
	if _, ok := metadata["clientId"]; !ok {
		t.Error("camelCase 'clientId' should exist in raw format")
	}
}

// TestIntegration_ConversionPipelineProducesValidCredential verifies the full
// pipeline: raw Kiro export → ParseKiroInput → ConvertKiroCredential produces
// a credential that the server's auth system can work with.
func TestIntegration_ConversionPipelineProducesValidCredential(t *testing.T) {
	// Simulate the raw Kiro Account Manager export (array format)
	rawExport := `[
  {
    "refreshToken": "aorAAAAA_test_refresh",
    "clientId": "test-client-id-base64",
    "clientSecret": "eyJ0ZXN0IjoiY2xpZW50X3NlY3JldCJ9",
    "region": "us-east-1",
    "startUrl": "https://d-test.awsapps.com/start/",
    "provider": "Enterprise",
    "machineId": "test-machine"
  }
]`

	// Step 1: Parse the raw input
	creds, err := ParseKiroInput([]byte(rawExport))
	if err != nil {
		t.Fatalf("ParseKiroInput failed: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}

	// Step 2: Convert to internal format
	internal := ConvertKiroCredential(creds[0])

	// Step 3: Verify all required server-side fields
	if internal.Type != "kiro" {
		t.Errorf("Type = %q, want %q", internal.Type, "kiro")
	}
	if internal.RefreshToken != "aorAAAAA_test_refresh" {
		t.Errorf("RefreshToken not preserved: %q", internal.RefreshToken)
	}
	if internal.ClientID != "test-client-id-base64" {
		t.Errorf("ClientID not preserved: %q", internal.ClientID)
	}
	if internal.ClientSecret != "eyJ0ZXN0IjoiY2xpZW50X3NlY3JldCJ9" {
		t.Errorf("ClientSecret not preserved: %q", internal.ClientSecret)
	}
	if internal.Region != "us-east-1" {
		t.Errorf("Region not preserved: %q", internal.Region)
	}
	if internal.StartURL != "https://d-test.awsapps.com/start/" {
		t.Errorf("StartURL not preserved: %q", internal.StartURL)
	}

	// Step 4: access_token intentionally empty — server's refresh manager fills it
	if internal.AccessToken != "" {
		t.Errorf("AccessToken should be empty after conversion, got %q", internal.AccessToken)
	}

	// Step 5: ExpiresAt set to past — triggers immediate refresh
	if internal.ExpiresAt != "2020-01-01T00:00:00Z" {
		t.Errorf("ExpiresAt should be in the past to trigger refresh, got %q", internal.ExpiresAt)
	}

	// Step 6: Verify the converted JSON has snake_case keys
	data, err := json.Marshal(internal)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var serverMetadata map[string]any
	if err := json.Unmarshal(data, &serverMetadata); err != nil {
		t.Fatalf("server-side unmarshal failed: %v", err)
	}

	// These are the keys the server's kiro_usage.go checks
	if _, ok := serverMetadata["type"]; !ok {
		t.Error("converted JSON missing 'type' key")
	}
	if _, ok := serverMetadata["refresh_token"]; !ok {
		t.Error("converted JSON missing 'refresh_token' key")
	}
	if _, ok := serverMetadata["client_id"]; !ok {
		t.Error("converted JSON missing 'client_id' key")
	}
	if _, ok := serverMetadata["client_secret"]; !ok {
		t.Error("converted JSON missing 'client_secret' key")
	}
}

// TestIntegration_ConvertedCredentialSimulateServerLoad simulates how the server's
// registerAuthFromFile would load the converted credential and populate auth.Metadata.
func TestIntegration_ConvertedCredentialSimulateServerLoad(t *testing.T) {
	ext := KiroExternalCredential{
		RefreshToken: "rt-integration-test",
		ClientID:     "cid-integration",
		ClientSecret: "cs-integration",
		Region:       "us-east-1",
		StartURL:     "https://d-test.awsapps.com/start/",
		Provider:     "Enterprise",
		Email:        "test@outlook.com",
	}

	internal := ConvertKiroCredential(ext)
	data, err := json.MarshalIndent(internal, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Simulate registerAuthFromFile: unmarshal into map[string]any
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("server-side unmarshal failed: %v", err)
	}

	// Simulate kiro_usage.go provider detection (line 50)
	provider, _ := metadata["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(provider), "kiro") {
		t.Errorf("provider detection failed: type = %q, want 'kiro'", provider)
	}

	// Simulate kiro_usage.go access_token check (line 57)
	accessToken, _ := metadata["access_token"].(string)
	if accessToken != "" {
		t.Error("freshly converted credential should have empty access_token")
	}

	// Verify refresh_token is available for the refresh manager
	refreshToken, _ := metadata["refresh_token"].(string)
	if refreshToken == "" {
		t.Error("refresh_token must be present for token refresh to work")
	}

	// Verify client credentials are available for SSO token creation
	clientID, _ := metadata["client_id"].(string)
	clientSecret, _ := metadata["client_secret"].(string)
	if clientID == "" || clientSecret == "" {
		t.Error("client_id and client_secret must be present for SSO refresh")
	}

	// Verify email is preserved for display
	email, _ := metadata["email"].(string)
	if email != "test@outlook.com" {
		t.Errorf("email = %q, want %q", email, "test@outlook.com")
	}
}

// TestIntegration_MultipleCredentialsFromArray verifies that a multi-account
// Kiro export produces unique filenames and valid credentials for each account.
func TestIntegration_MultipleCredentialsFromArray(t *testing.T) {
	rawExport := `[
  {
    "refreshToken": "rt-account-1",
    "clientId": "cid-1-xxxx",
    "clientSecret": "cs-1",
    "region": "us-east-1",
    "startUrl": "https://d-c367640ad3.awsapps.com/start/",
    "provider": "Enterprise",
    "machineId": "",
    "email": ""
  },
  {
    "refreshToken": "rt-account-2",
    "clientId": "cid-2-yyyy",
    "clientSecret": "cs-2",
    "region": "us-east-1",
    "startUrl": "https://d-c367640ad3.awsapps.com/start/",
    "provider": "Enterprise",
    "machineId": "",
    "email": ""
  }
]`

	creds, err := ParseKiroInput([]byte(rawExport))
	if err != nil {
		t.Fatalf("ParseKiroInput failed: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}

	// Verify unique filenames when same startUrl but different clientIds
	name1 := GenerateKiroFileName(creds[0], 0)
	name2 := GenerateKiroFileName(creds[1], 1)
	if name1 == name2 {
		t.Errorf("duplicate filenames for different accounts: %q", name1)
	}

	// Verify each credential converts correctly
	for i, cred := range creds {
		internal := ConvertKiroCredential(cred)
		if internal.Type != "kiro" {
			t.Errorf("cred[%d]: Type = %q", i, internal.Type)
		}
		if internal.RefreshToken == "" {
			t.Errorf("cred[%d]: RefreshToken should not be empty", i)
		}
		if internal.ClientID == "" {
			t.Errorf("cred[%d]: ClientID should not be empty", i)
		}
	}
}

// TestIntegration_DiagnoseRawFileProblems provides a diagnostic function that
// identifies all issues with a raw credential file, similar to what a health
// check endpoint would do.
func TestIntegration_DiagnoseRawFileProblems(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		problems []string
	}{
		{
			name: "raw kiro array export",
			input: `[{"refreshToken":"rt","clientId":"cid","clientSecret":"cs",
				"region":"us-east-1","startUrl":"https://x.awsapps.com/start/",
				"provider":"Enterprise","machineId":""}]`,
			problems: []string{"JSON array", "camelCase"},
		},
		{
			name:     "camelCase single object without type",
			input:    `{"refreshToken":"rt","clientId":"cid","clientSecret":"cs","region":"us-east-1"}`,
			problems: []string{"camelCase", "missing type"},
		},
		{
			name: "valid converted credential",
			input: `{"type":"kiro","access_token":"","refresh_token":"rt",
				"client_id":"cid","client_secret":"cs","auth_method":"idc",
				"region":"us-east-1","expires_at":"2020-01-01T00:00:00Z"}`,
			problems: []string{}, // no problems
		},
		{
			name:     "missing refresh_token in snake_case format",
			input:    `{"type":"kiro","access_token":"","refresh_token":"","client_id":"cid"}`,
			problems: []string{"missing refresh_token"},
		},
		{
			name:     "missing client credentials",
			input:    `{"type":"kiro","refresh_token":"rt","client_id":"","client_secret":""}`,
			problems: []string{"missing client_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := diagnoseCredentialFile([]byte(tt.input))

			for _, expected := range tt.problems {
				found := false
				for _, p := range problems {
					if strings.Contains(strings.ToLower(p), strings.ToLower(expected)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected problem containing %q, got problems: %v", expected, problems)
				}
			}

			if len(tt.problems) == 0 && len(problems) > 0 {
				t.Errorf("expected no problems, got: %v", problems)
			}
		})
	}
}

// diagnoseCredentialFile checks a raw credential file for common issues.
func diagnoseCredentialFile(data []byte) []string {
	var problems []string
	data = []byte(strings.TrimSpace(string(data)))

	if len(data) == 0 {
		return []string{"empty file"}
	}

	// Check if it's a JSON array (server expects object)
	if data[0] == '[' {
		problems = append(problems, "JSON array format: server expects a single JSON object, not an array")
	}

	// Try to parse as map (what the server does)
	var metadata map[string]any
	if err := json.Unmarshal(data, &metadata); err != nil {
		// If it's an array, try to extract the first element for further diagnosis
		var arr []map[string]any
		if arrErr := json.Unmarshal(data, &arr); arrErr == nil && len(arr) > 0 {
			metadata = arr[0]
		} else {
			problems = append(problems, "invalid JSON: "+err.Error())
			return problems
		}
	}

	// Check for camelCase fields (indicates raw Kiro Account Manager export)
	if _, ok := metadata["refreshToken"]; ok {
		problems = append(problems, "camelCase fields detected: file uses Kiro Account Manager format, needs conversion via 'upload-kiro'")
	}

	// Check for required server-side fields
	if t, _ := metadata["type"].(string); t == "" {
		if _, hasCamel := metadata["provider"]; hasCamel {
			problems = append(problems, "missing type field: raw export uses 'provider' but server expects 'type: kiro'")
		} else {
			problems = append(problems, "missing type field")
		}
	}

	// In snake_case format, check for essential fields
	if _, ok := metadata["refresh_token"]; ok {
		rt, _ := metadata["refresh_token"].(string)
		if rt == "" {
			problems = append(problems, "missing refresh_token: token refresh will fail")
		}
		cid, _ := metadata["client_id"].(string)
		if cid == "" {
			problems = append(problems, "missing client_id: SSO refresh requires client credentials")
		}
	}

	return problems
}

// TestIntegration_EndToEndFileConversion tests the full file-based workflow:
// read raw file → parse → convert → serialize → verify server compatibility.
func TestIntegration_EndToEndFileConversion(t *testing.T) {
	dir := t.TempDir()

	// Write a raw Kiro Account Manager export file
	rawContent := `[
  {
    "refreshToken": "aorAAAAA_e2e_test_token",
    "clientId": "E2ETest-clientId-base64encoded",
    "clientSecret": "eyJlMmUiOiJ0ZXN0In0.signature",
    "region": "us-east-1",
    "startUrl": "https://d-e2etest.awsapps.com/start/",
    "provider": "Enterprise",
    "machineId": "test-machine-id"
  }
]`

	rawPath := filepath.Join(dir, "raw-export.json")
	if err := os.WriteFile(rawPath, []byte(rawContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Step 1: Read the raw file
	data, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	// Step 2: Parse
	creds, err := ParseKiroInput(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Step 3: Convert and write each credential
	for i, cred := range creds {
		internal := ConvertKiroCredential(cred)
		fileName := GenerateKiroFileName(cred, i)

		convertedData, err := json.MarshalIndent(internal, "", "  ")
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		convertedPath := filepath.Join(dir, fileName)
		if err := os.WriteFile(convertedPath, convertedData, 0600); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		// Step 4: Verify the file can be loaded by the server
		serverData, err := os.ReadFile(convertedPath)
		if err != nil {
			t.Fatalf("server read failed: %v", err)
		}

		metadata := make(map[string]any)
		if err := json.Unmarshal(serverData, &metadata); err != nil {
			t.Fatalf("server unmarshal failed: %v", err)
		}

		// Verify no problems detected
		problems := diagnoseCredentialFile(serverData)
		if len(problems) > 0 {
			t.Errorf("converted file still has problems: %v", problems)
		}

		// Verify provider detection works
		providerType, _ := metadata["type"].(string)
		if providerType != "kiro" {
			t.Errorf("type = %q, want 'kiro'", providerType)
		}

		// Verify refresh_token is present for background refresh
		rt, _ := metadata["refresh_token"].(string)
		if rt == "" {
			t.Error("refresh_token missing after conversion")
		}
	}
}
