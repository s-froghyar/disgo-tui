package client

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/discogs"
)

// Build-time variables (set during compilation)
var (
	// These will be set via -ldflags during build in CI/CD
	defaultConsumerKey    = ""
	defaultConsumerSecret = ""
	version               = "dev"
)

const (
	defaultTimeout = 30 * time.Second
	configFileName = "discogs_tui_config.enc"
	defaultPort    = "8080"
)

var (
	ErrTokenGenerationFailed = errors.New("failed to generate OAuth token")
)

type DiscogsIdentity struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	ResourceUrl  string `json:"resource_url"`
	ConsumerName string `json:"consumer_name"`
}

type TokenConfig struct {
	Token       string `json:"token"`
	TokenSecret string `json:"token_secret"`
}

type DiscogsClient struct {
	*http.Client

	Identity DiscogsIdentity

	// OAuth configuration and state
	config            oauth1.Config
	consumerKey       string
	consumerSecretKey string
	localPort         string
	requestToken      string
	requestSecret     string
	handlingRedirect  bool
	doneVerifying     bool
	token             *oauth1.Token
	oauthComplete     chan error
}

type customTransport struct {
	Transport http.RoundTripper
	client    *DiscogsClient
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.client.token == nil {
		return nil, errors.New("no OAuth token available")
	}

	ts := time.Now().Unix()
	req.Header.Set("User-Agent", fmt.Sprintf("DiscosTUI/%s", version))
	req.Header.Set("Authorization", fmt.Sprintf(`OAuth oauth_consumer_key="%v",oauth_nonce="%v",oauth_token="%v",oauth_signature="%v&%v",oauth_signature_method="PLAINTEXT",oauth_timestamp="%v"`,
		t.client.consumerKey, ts, t.client.token.Token, t.client.consumerSecretKey, t.client.token.TokenSecret, ts))

	return t.Transport.RoundTrip(req)
}

// validateConfig validates that all required configuration is present
func (c *DiscogsClient) validateConfig() error {
	if c.consumerKey == "" {
		return errors.New("no API credentials available - this appears to be a development build")
	}
	if c.consumerSecretKey == "" {
		return errors.New("incomplete API credentials - this appears to be a development build")
	}
	return nil
}

// getAvailablePort finds an available port for the OAuth callback
func getAvailablePort() string {
	// Try default port first
	if isPortAvailable(defaultPort) {
		return defaultPort
	}

	// Try some common ports
	commonPorts := []string{"8081", "8082", "8083", "8084", "8085"}
	for _, port := range commonPorts {
		if isPortAvailable(port) {
			return port
		}
	}

	// Fall back to default and let the OS handle conflicts
	return defaultPort
}

func isPortAvailable(port string) bool {
	// Simple check - try to listen on the port briefly
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// New returns an authenticated http.Client for the Discogs API
func New() (*DiscogsClient, error) {
	return NewWithContext(context.Background())
}

// NewWithContext returns an authenticated http.Client for the Discogs API with context support
func NewWithContext(ctx context.Context) (*DiscogsClient, error) {
	c := &DiscogsClient{
		Client: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	// Initialize API credentials
	// Priority: 1. Environment variables (for development)
	//          2. Build-time embedded credentials (for releases)
	c.consumerKey = os.Getenv("DISCOGS_API_CONSUMER_KEY")
	c.consumerSecretKey = os.Getenv("DISCOGS_API_CONSUMER_SECRET")

	if c.consumerKey == "" && defaultConsumerKey != "" {
		c.consumerKey = defaultConsumerKey
		c.consumerSecretKey = defaultConsumerSecret
		fmt.Println("Using embedded API credentials")
	}

	// Set OAuth callback port
	c.localPort = os.Getenv("LOCAL_PORT")
	if c.localPort == "" {
		c.localPort = getAvailablePort()
	}

	// Validate configuration
	if err := c.validateConfig(); err != nil {
		return nil, err
	}

	fmt.Println("🎵 Welcome to Discogs TUI!")
	fmt.Println("Looking for existing authentication...")

	// Try to load existing tokens from secure storage
	savedToken, err := c.loadTokensSecurely()
	if err == nil && savedToken != nil {
		c.token = savedToken
		fmt.Println("✓ Found existing authentication")
	} else {
		fmt.Println("No existing authentication found")
		fmt.Println("Starting Discogs authentication...")
		fmt.Println("This is a one-time setup - your credentials will be saved securely")

		// Generate new tokens via OAuth
		err := c.generateDiscogsTokenWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
		}

		// Save newly generated tokens
		if err := c.saveTokensSecurely(); err != nil {
			fmt.Printf("Warning: Failed to save authentication securely: %v\n", err)
		} else {
			fmt.Println("✓ Authentication saved securely - you won't need to re-authenticate!")
		}
	}

	// Set up custom transport
	c.Transport = &customTransport{
		Transport: http.DefaultTransport,
		client:    c,
	}

	fmt.Println("Verifying authentication with Discogs...")
	err = c.getIdentityWithContext(ctx)
	if err != nil {
		// If auth fails, token might be invalid - try to re-authenticate
		fmt.Printf("Authentication verification failed: %v\n", err)
		fmt.Println("Re-authenticating...")

		// Clear invalid tokens
		c.token = nil

		// Try OAuth flow again
		err := c.generateDiscogsTokenWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}

		// Save new tokens
		if err := c.saveTokensSecurely(); err != nil {
			fmt.Printf("Warning: Failed to save authentication: %v\n", err)
		}

		// Verify again
		err = c.getIdentityWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("authentication still failing: %w", err)
		}
	}

	fmt.Printf("✓ Successfully authenticated as: %v\n", c.Identity.Username)
	fmt.Println("Loading your Discogs data...")
	return c, nil
}

// generateDiscogsTokenWithContext generates OAuth tokens with context support
func (c *DiscogsClient) generateDiscogsTokenWithContext(ctx context.Context) error {
	c.config = oauth1.Config{
		ConsumerKey:    c.consumerKey,
		ConsumerSecret: c.consumerSecretKey,
		CallbackURL:    "http://localhost:" + c.localPort,
		Endpoint:       discogs.Endpoint,
	}

	// Get request token
	token, secret, err := c.config.RequestToken()
	if err != nil {
		return fmt.Errorf("failed to get request token: %w", err)
	}

	c.requestToken = token
	c.requestSecret = secret

	authorizationUrl, err := c.config.AuthorizationURL(c.requestToken)
	if err != nil {
		return fmt.Errorf("failed to get authorization URL: %w", err)
	}

	// Initialize completion channel
	c.oauthComplete = make(chan error, 1)

	// Create OAuth callback server
	server := &http.Server{
		Addr:    ":" + c.localPort,
		Handler: http.HandlerFunc(c.handleRedirect),
	}

	// Start server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case c.oauthComplete <- fmt.Errorf("server error: %w", err):
			default:
			}
		}
	}()

	// Open browser automatically if possible
	fmt.Printf("\n🔐 Please authenticate with Discogs:\n")
	fmt.Printf("   %s\n\n", authorizationUrl.String())

	if err := openBrowser(authorizationUrl.String()); err == nil {
		fmt.Println("✓ Opened authentication page in your browser")
	} else {
		fmt.Println("Please copy the URL above into your browser")
	}

	fmt.Printf("Waiting for authentication (listening on port %s)...\n", c.localPort)

	// Wait for completion
	select {
	case <-ctx.Done():
		server.Close()
		return ctx.Err()
	case err := <-c.oauthComplete:
		server.Close()
		if err != nil {
			return err
		}
		fmt.Println("✓ Authentication successful!")
		return nil
	case <-time.After(5 * time.Minute):
		server.Close()
		return errors.New("authentication timed out after 5 minutes")
	}
}

// openBrowser attempts to open the URL in the user's default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)

	exec := exec.Command(cmd, args...)
	return exec.Start()
}

func (c *DiscogsClient) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if c.handlingRedirect || c.doneVerifying {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Authentication already in progress"))
		return
	}

	c.handlingRedirect = true
	defer func() { c.handlingRedirect = false }()

	// Get OAuth parameters
	receivedToken := r.URL.Query().Get("oauth_token")
	verificationCode := r.URL.Query().Get("oauth_verifier")

	// Validate token
	if receivedToken != c.requestToken {
		http.Error(w, "Invalid OAuth token", http.StatusBadRequest)
		select {
		case c.oauthComplete <- errors.New("invalid OAuth token"):
		default:
		}
		return
	}

	// Validate verification code
	if verificationCode == "" {
		http.Error(w, "No verification code received", http.StatusBadRequest)
		select {
		case c.oauthComplete <- errors.New("no verification code received"):
		default:
		}
		return
	}

	// Exchange for access token
	accessToken, accessSecret, err := c.config.AccessToken(c.requestToken, c.requestSecret, verificationCode)
	if err != nil {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		select {
		case c.oauthComplete <- fmt.Errorf("failed to get access token: %w", err):
		default:
		}
		return
	}

	c.token = oauth1.NewToken(accessToken, accessSecret)
	c.doneVerifying = true

	// Send success response
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Discogs TUI - Authentication Successful</title>
			<style>
				body { font-family: system-ui, sans-serif; text-align: center; padding: 50px; background: #f5f5f5; }
				.container { background: white; border-radius: 10px; padding: 40px; max-width: 500px; margin: 0 auto; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
				.success { color: #28a745; font-size: 24px; margin-bottom: 20px; }
				.message { color: #6c757d; font-size: 16px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="success">✓ Authentication Successful!</div>
				<div class="message">
					You can now close this window and return to your terminal.<br>
					Discogs TUI is ready to use!
				</div>
			</div>
		</body>
		</html>
	`))

	// Signal completion
	select {
	case c.oauthComplete <- nil:
	default:
	}
}

// Rest of the file remains the same (token storage, crypto functions, etc.)
// ... [include all the existing token storage, encryption, and API methods]
// getConfigDir returns the user's config directory
func (c *DiscogsClient) getConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appConfigDir := filepath.Join(configDir, "discogs-tui")
	if err := os.MkdirAll(appConfigDir, 0700); err != nil {
		return "", err
	}

	return appConfigDir, nil
}

// generateKey generates a key from the consumer secret for encryption
func (c *DiscogsClient) generateKey() []byte {
	// Use a portion of the consumer secret as encryption key
	// In production, you might want to use a proper key derivation function
	key := []byte(c.consumerSecretKey)
	if len(key) > 32 {
		key = key[:32]
	} else if len(key) < 32 {
		// Pad with zeros if too short
		padding := make([]byte, 32-len(key))
		key = append(key, padding...)
	}
	return key
}

// saveTokensSecurely saves OAuth tokens to an encrypted file
func (c *DiscogsClient) saveTokensSecurely() error {
	if c.token == nil {
		return errors.New("no token to save")
	}

	configDir, err := c.getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	tokenConfig := TokenConfig{
		Token:       c.token.Token,
		TokenSecret: c.token.TokenSecret,
	}

	// Marshal to JSON
	data, err := json.Marshal(tokenConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal token config: %w", err)
	}

	// Encrypt the data
	encryptedData, err := c.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt token data: %w", err)
	}

	// Write to file
	configFile := filepath.Join(configDir, configFileName)
	if err := os.WriteFile(configFile, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Println("OAuth tokens saved securely")
	return nil
}

// loadTokensSecurely loads OAuth tokens from an encrypted file
func (c *DiscogsClient) loadTokensSecurely() (*oauth1.Token, error) {
	configDir, err := c.getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	configFile := filepath.Join(configDir, configFileName)

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, errors.New("config file does not exist")
	}

	// Read encrypted data
	encryptedData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Decrypt the data
	data, err := c.decrypt(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token data: %w", err)
	}

	// Unmarshal from JSON
	var tokenConfig TokenConfig
	if err := json.Unmarshal(data, &tokenConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token config: %w", err)
	}

	return &oauth1.Token{
		Token:       tokenConfig.Token,
		TokenSecret: tokenConfig.TokenSecret,
	}, nil
}

// encrypt encrypts data using AES
func (c *DiscogsClient) encrypt(data []byte) ([]byte, error) {
	key := c.generateKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Encode to base64 for storage
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(encoded), nil
}

// decrypt decrypts data using AES
func (c *DiscogsClient) decrypt(data []byte) ([]byte, error) {
	key := c.generateKey()

	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// printTokensToConsole prints tokens to console as fallback
func (c *DiscogsClient) printTokensToConsole() {
	if c.token == nil {
		return
	}

	fmt.Printf(`
	OAuth tokens generated! Add the following to your .zshrc or .bashrc file 
	to save your auth token as an env variable:

	# .zshrc/.bashrc
	export DISCOGS_TOKEN="%v"
	export DISCOGS_TOKEN_SECRET="%v"
	`, c.token.Token, c.token.TokenSecret)
}

// getIdentityWithContext gets user identity with context support
func (c *DiscogsClient) getIdentityWithContext(ctx context.Context) error {
	path := "https://api.discogs.com/oauth/identity"

	req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("error at Get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&c.Identity)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}
	return nil
}

// getIdentity maintains backward compatibility
func (c *DiscogsClient) getIdentity() error {
	return c.getIdentityWithContext(context.Background())
}

// GetThumbImageWithContext gets thumbnail image with context support
func (c *DiscogsClient) GetThumbImageWithContext(ctx context.Context, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error at Get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode the body into an image.Image
	img, err := jpeg.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error decoding image: %w", err)
	}

	return img, nil
}

// GetThumbImage maintains backward compatibility
func (c *DiscogsClient) GetThumbImage(url string) (image.Image, error) {
	return c.GetThumbImageWithContext(context.Background(), url)
}
