package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KiroExternalCredential is the camelCase format exported by Kiro Account Manager.
type KiroExternalCredential struct {
	RefreshToken string `json:"refreshToken"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Region       string `json:"region"`
	StartURL     string `json:"startUrl"`
	Provider     string `json:"provider"`
	MachineID    string `json:"machineId"`
	Email        string `json:"email"`
}

// KiroInternalCredential is the snake_case format expected by CLIProxyAPI's auth system.
type KiroInternalCredential struct {
	Type         string `json:"type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	AuthMethod   string `json:"auth_method"`
	Provider     string `json:"provider"`
	Region       string `json:"region,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	ClientIDHash string `json:"client_id_hash,omitempty"`
	StartURL     string `json:"start_url,omitempty"`
	Email        string `json:"email,omitempty"`
}

// KiroSSOTokenFile represents the AWS SSO token cache file (camelCase).
// Located at e.g. ~/.aws/sso/cache/kiro-auth-token.json
type KiroSSOTokenFile struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
	AuthMethod   string `json:"authMethod"`
	Provider     string `json:"provider"`
	Region       string `json:"region"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	ClientIDHash string `json:"clientIdHash"`
	StartURL     string `json:"startUrl"`
	Email        string `json:"email"`
}

// KiroSSOClientFile represents the AWS SSO device registration file.
// Located at e.g. ~/.aws/sso/cache/{clientIdHash}.json
type KiroSSOClientFile struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// ParseKiroInput accepts JSON that is either a single object or an array of objects.
func ParseKiroInput(data []byte) ([]KiroExternalCredential, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	if data[0] == '[' {
		var arr []KiroExternalCredential
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, fmt.Errorf("parsing JSON array: %w", err)
		}
		return arr, nil
	}

	var single KiroExternalCredential
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("parsing JSON object: %w", err)
	}
	return []KiroExternalCredential{single}, nil
}

// ConvertKiroCredential converts from external (Kiro Account Manager) to internal format.
func ConvertKiroCredential(ext KiroExternalCredential) KiroInternalCredential {
	return KiroInternalCredential{
		Type:         "kiro",
		AccessToken:  "",
		RefreshToken: ext.RefreshToken,
		ExpiresAt:    "2020-01-01T00:00:00Z",
		AuthMethod:   "idc",
		Provider:     ext.Provider,
		Region:       ext.Region,
		ClientID:     ext.ClientID,
		ClientSecret: ext.ClientSecret,
		StartURL:     ext.StartURL,
		Email:        ext.Email,
	}
}

// GenerateKiroFileName produces a filename like kiro-<provider>-<identifier>.json.
// Priority: email > startUrl hostname > fallback index.
func GenerateKiroFileName(cred KiroExternalCredential, index int) string {
	provider := cred.Provider
	if provider == "" {
		provider = "unknown"
	}

	var identifier string
	switch {
	case cred.Email != "":
		identifier = sanitizeForFilename(cred.Email)
	case cred.StartURL != "":
		if u, err := url.Parse(cred.StartURL); err == nil && u.Hostname() != "" {
			identifier = sanitizeForFilename(u.Hostname())
		}
	}
	if identifier == "" {
		identifier = fmt.Sprintf("%d", index)
	}

	return fmt.Sprintf("kiro-%s-%s.json", sanitizeForFilename(provider), identifier)
}

// UploadKiroCredential POSTs the credential JSON to the management API.
func UploadKiroCredential(server, apiKey, fileName string, cred KiroInternalCredential) error {
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling credential: %w", err)
	}

	uploadURL := strings.TrimRight(server, "/") + "/v0/management/auth-files?name=" + url.QueryEscape(fileName)

	req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ReadKiroInputFromStdin prompts on stderr and reads stdin until EOF.
func ReadKiroInputFromStdin() ([]byte, error) {
	fmt.Fprintln(os.Stderr, "Paste Kiro credential JSON (press Ctrl+D when done):")
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return data, nil
}

// ReadKiroSSOFiles reads the AWS SSO token file and its companion client file,
// then merges them into a KiroInternalCredential.
// If clientPath is empty and the token file contains a clientIdHash,
// the client file is auto-discovered in the same directory as the token file.
func ReadKiroSSOFiles(tokenPath, clientPath string) (KiroInternalCredential, error) {
	tokenPath = ResolvePath(tokenPath)

	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		return KiroInternalCredential{}, fmt.Errorf("reading token file: %w", err)
	}

	var token KiroSSOTokenFile
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return KiroInternalCredential{}, fmt.Errorf("parsing token file: %w", err)
	}

	if token.RefreshToken == "" {
		return KiroInternalCredential{}, fmt.Errorf("token file missing refreshToken")
	}

	// Merge clientId/clientSecret from the companion client file
	clientID := token.ClientID
	clientSecret := token.ClientSecret

	if (clientID == "" || clientSecret == "") && token.ClientIDHash != "" {
		if clientPath == "" {
			// Auto-discover: same directory as token file, named {clientIdHash}.json
			clientPath = filepath.Join(filepath.Dir(tokenPath), token.ClientIDHash+".json")
		} else {
			clientPath = ResolvePath(clientPath)
		}

		clientData, err := os.ReadFile(clientPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot read client file %s: %v\n", clientPath, err)
			fmt.Fprintln(os.Stderr, "Uploading without client_id/client_secret")
		} else {
			var client KiroSSOClientFile
			if err := json.Unmarshal(clientData, &client); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot parse client file: %v\n", err)
			} else {
				clientID = client.ClientID
				clientSecret = client.ClientSecret
			}
		}
	} else if clientPath != "" {
		// Explicit client file provided even though token already has credentials
		clientPath = ResolvePath(clientPath)
		clientData, err := os.ReadFile(clientPath)
		if err != nil {
			return KiroInternalCredential{}, fmt.Errorf("reading client file: %w", err)
		}
		var client KiroSSOClientFile
		if err := json.Unmarshal(clientData, &client); err != nil {
			return KiroInternalCredential{}, fmt.Errorf("parsing client file: %w", err)
		}
		clientID = client.ClientID
		clientSecret = client.ClientSecret
	}

	return KiroInternalCredential{
		Type:         "kiro",
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
		AuthMethod:   token.AuthMethod,
		Provider:     token.Provider,
		Region:       token.Region,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		ClientIDHash: token.ClientIDHash,
		StartURL:     token.StartURL,
		Email:        token.Email,
	}, nil
}

// ResolvePath expands ~ to the home directory and converts relative paths to absolute.
func ResolvePath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func sanitizeForFilename(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
