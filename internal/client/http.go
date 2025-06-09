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
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/discogs"
	"github.com/joho/godotenv"
)

const (
	defaultTimeout = 30 * time.Second
	configFileName = "discogs_tui_config.enc"
)

var (
	ErrMissingConsumerKey    = errors.New("DISCOGS_API_CONSUMER_KEY environment variable is required")
	ErrMissingConsumerSecret = errors.New("DISCOGS_API_CONSUMER_SECRET environment variable is required")
	ErrMissingLocalPort      = errors.New("LOCAL_PORT environment variable is required")
	ErrInvalidLocalPort      = errors.New("LOCAL_PORT must be a valid port number")
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

	// Add this field for OAuth completion signaling
	oauthComplete chan error
}

type customTransport struct {
	Transport http.RoundTripper
	client    *DiscogsClient // Reference to client for accessing token/keys
}

// RoundTrip implements the RoundTripper interface to add the necessary headers for the Discogs API
func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.client.token == nil {
		return nil, errors.New("no OAuth token available")
	}

	ts := time.Now().Unix()
	req.Header.Set("Authorization", fmt.Sprintf(`OAuth oauth_consumer_key="%v",oauth_nonce="%v",oauth_token="%v",oauth_signature="%v&%v",oauth_signature_method="PLAINTEXT",oauth_timestamp="%v"`,
		t.client.consumerKey, ts, t.client.token.Token, t.client.consumerSecretKey, t.client.token.TokenSecret, ts))

	return t.Transport.RoundTrip(req)
}

// validateConfig validates that all required configuration is present and valid
func (c *DiscogsClient) validateConfig() error {
	if c.consumerKey == "" {
		return ErrMissingConsumerKey
	}
	if c.consumerSecretKey == "" {
		return ErrMissingConsumerSecret
	}
	if c.localPort == "" {
		return ErrMissingLocalPort
	}

	// Basic port validation (should be numeric and reasonable range)
	if len(c.localPort) < 1 || len(c.localPort) > 5 {
		return ErrInvalidLocalPort
	}

	return nil
}

// New returns an authenticated http.Client for the Discogs API
func New() (*DiscogsClient, error) {
	return NewWithContext(context.Background())
}

// NewWithContext returns an authenticated http.Client for the Discogs API with context support
func NewWithContext(ctx context.Context) (*DiscogsClient, error) {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Error loading .env file, using system environment variables")
	}

	c := &DiscogsClient{
		Client: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	// Initialize client fields from environment
	c.consumerKey = os.Getenv("DISCOGS_API_CONSUMER_KEY")
	c.consumerSecretKey = os.Getenv("DISCOGS_API_CONSUMER_SECRET")
	c.localPort = os.Getenv("LOCAL_PORT")

	// Validate configuration
	if err := c.validateConfig(); err != nil {
		return nil, err
	}

	fmt.Println("Looking for existing OAuth tokens...")

	// Try to load existing tokens from secure storage first
	savedToken, err := c.loadTokensSecurely()
	if err == nil && savedToken != nil {
		c.token = savedToken
		fmt.Println("✓ Loaded existing OAuth tokens from secure storage")
	} else {
		fmt.Printf("Secure storage unavailable (%v), checking environment variables...\n", err)

		// Fall back to environment variables
		t := os.Getenv("DISCOGS_TOKEN")
		s := os.Getenv("DISCOGS_TOKEN_SECRET")

		if t != "" && s != "" {
			c.token = &oauth1.Token{
				Token:       t,
				TokenSecret: s,
			}
			fmt.Println("✓ Loaded OAuth tokens from environment variables")

			// Save to secure storage for future use
			if err := c.saveTokensSecurely(); err != nil {
				fmt.Printf("Warning: Failed to save tokens securely: %v\n", err)
			} else {
				fmt.Println("✓ Saved tokens to secure storage for future use")
			}
		} else {
			fmt.Println("No existing tokens found, starting OAuth flow...")

			// Generate new tokens
			err := c.generateDiscogsTokenWithContext(ctx)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
			}

			// Save newly generated tokens
			if err := c.saveTokensSecurely(); err != nil {
				fmt.Printf("Warning: Failed to save tokens securely: %v\n", err)
				fmt.Println("You can manually save these tokens to avoid re-authentication:")
				c.printTokensToConsole()
			} else {
				fmt.Println("✓ OAuth tokens saved securely - you won't need to re-authenticate next time!")
			}
		}
	}

	// Set up custom transport with reference to client
	c.Transport = &customTransport{
		Transport: http.DefaultTransport,
		client:    c,
	}

	fmt.Println("Verifying authentication with Discogs API...")
	err = c.getIdentityWithContext(ctx)
	if err != nil {
		// If auth fails, token might be invalid - try to re-authenticate
		fmt.Printf("Authentication failed: %v\n", err)
		fmt.Println("Tokens may be invalid, starting fresh OAuth flow...")

		// Clear invalid tokens
		c.token = nil

		// Try OAuth flow again
		err := c.generateDiscogsTokenWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}

		// Save new tokens
		if err := c.saveTokensSecurely(); err != nil {
			fmt.Printf("Warning: Failed to save tokens securely: %v\n", err)
			c.printTokensToConsole()
		}

		// Verify again
		err = c.getIdentityWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("authentication still failing after re-auth: %w", err)
		}
	}

	fmt.Printf("✓ Successfully authenticated as user: %v\n", c.Identity.Username)
	return c, nil
}

// generateDiscogsTokenWithContext generates OAuth tokens with context support for timeouts
func (c *DiscogsClient) generateDiscogsTokenWithContext(ctx context.Context) error {
	c.config = oauth1.Config{
		ConsumerKey:    c.consumerKey,
		ConsumerSecret: c.consumerSecretKey,
		CallbackURL:    "http://localhost:" + c.localPort,
		Endpoint:       discogs.Endpoint,
	}

	// Get request token with context
	token, secret, err := c.config.RequestToken()
	if err != nil {
		return fmt.Errorf("error at RequestToken: %w", err)
	}

	c.requestToken = token
	c.requestSecret = secret

	authorizationUrl, err := c.config.AuthorizationURL(c.requestToken)
	if err != nil {
		return fmt.Errorf("error at AuthorizationURL: %w", err)
	}

	// Initialize the completion channel
	c.oauthComplete = make(chan error, 1)

	// Create a server with context
	server := &http.Server{
		Addr:    ":" + c.localPort,
		Handler: http.HandlerFunc(c.handleRedirect),
	}

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Only send error if OAuth hasn't completed yet
			select {
			case c.oauthComplete <- fmt.Errorf("error starting server: %w", err):
			default:
				// Channel already closed or has a value
			}
		}
	}()

	fmt.Printf("Authorization URL: %v\n", authorizationUrl.String())
	fmt.Println("Please verify yourself! if you are not redirected, please visit the link above")
	fmt.Println("Waiting for OAuth completion...")

	// Wait for either context cancellation, OAuth completion, or timeout
	select {
	case <-ctx.Done():
		server.Close()
		return ctx.Err()
	case err := <-c.oauthComplete:
		// OAuth completed (either successfully or with error)
		server.Close()
		if err != nil {
			return err
		}
		fmt.Println("OAuth completed successfully!")
		return nil
	case <-time.After(5 * time.Minute): // 5 minute timeout for OAuth flow
		server.Close()
		return errors.New("OAuth flow timed out after 5 minutes")
	}
}

func (c *DiscogsClient) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if c.handlingRedirect || c.doneVerifying {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("Already handling a request"))
		if err != nil {
			fmt.Println("Error writing response:", err)
		}
		return
	}
	c.handlingRedirect = true
	defer func() { c.handlingRedirect = false }()

	var verificationCode string
	queryParams := r.URL.Query()

	// Validate OAuth token matches
	receivedToken := queryParams.Get("oauth_token")
	if receivedToken != c.requestToken {
		fmt.Printf("Token mismatch: expected %s, got %s\n", c.requestToken, receivedToken)
		http.Error(w, "Invalid OAuth token", http.StatusBadRequest)
		// Signal error
		select {
		case c.oauthComplete <- errors.New("invalid OAuth token"):
		default:
		}
		return
	}

	verificationCode = queryParams.Get("oauth_verifier")

	// Debug logging
	for key, values := range queryParams {
		for _, value := range values {
			fmt.Printf("Query parameter %s has value %s\n", key, value)
		}
	}

	// Validate that we have the verification code
	if verificationCode == "" {
		fmt.Println("No verification code received")
		http.Error(w, "No verification code", http.StatusBadRequest)
		// Signal error
		select {
		case c.oauthComplete <- errors.New("no verification code received"):
		default:
		}
		return
	}

	// use request token to get access token
	fmt.Printf("Verification Code: %v\n", verificationCode)
	accessToken, accessSecret, err := c.config.AccessToken(c.requestToken, c.requestSecret, verificationCode)
	if err != nil {
		fmt.Printf("Error at AccessToken: %v\n", err)
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		// Signal error
		select {
		case c.oauthComplete <- fmt.Errorf("failed to get access token: %w", err):
		default:
		}
		return
	}

	c.token = oauth1.NewToken(accessToken, accessSecret)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Authentication successful! You can close this window."))
	if err != nil {
		fmt.Println("Error writing response:", err)
	}

	c.doneVerifying = true

	// Signal successful completion
	select {
	case c.oauthComplete <- nil: // nil means success
		fmt.Println("OAuth completion signaled successfully")
	default:
		fmt.Println("OAuth completion channel already closed or full")
	}
}

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
