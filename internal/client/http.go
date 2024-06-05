package client

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/discogs"
	"github.com/joho/godotenv"
)

var (
	config            oauth1.Config
	consumerKey       string
	consumerSecretKey string
	localPort         string
	requestToken      string
	requestSecret     string
	handlingRedirect  bool = false
	doneVerifying     bool = false
	token             *oauth1.Token
)

type DiscogsIdentity struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	ResourceUrl  string `json:"resource_url"`
	ConsumerName string `json:"consumer_name"`
}

type DiscogsClient struct {
	*http.Client

	Identity DiscogsIdentity
}

type customTransport struct {
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface to add the necessary headers for the Discogs API
func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ts := time.Now().Unix()
	req.Header.Set("Authorization", fmt.Sprintf(`OAuth oauth_consumer_key="%v",oauth_nonce="%v",oauth_token="%v",oauth_signature="%v&%v",oauth_signature_method="PLAINTEXT",oauth_timestamp="%v"`, consumerKey, ts, token.Token, consumerSecretKey, token.TokenSecret, ts))

	return t.Transport.RoundTrip(req)
}

// New returns an authenticated http.Client for the Discogs API
func New() *DiscogsClient {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		panic(err)
	}
	consumerKey = os.Getenv("DISCOGS_API_CONSUMER_KEY")
	consumerSecretKey = os.Getenv("DISCOGS_API_CONSUMER_SECRET")
	t := os.Getenv("DISCOGS_TOKEN")
	s := os.Getenv("DISCOGS_TOKEN_SECRET")
	if t == "" || s == "" {
		generateDiscogsToken()
	} else {
		token = &oauth1.Token{
			Token:       t,
			TokenSecret: s,
		}
	}

	c := &DiscogsClient{
		Client: &http.Client{},
	}
	c.Transport = &customTransport{
		Transport: http.DefaultTransport,
	}

	err = c.getIdentity()
	if err != nil {
		log.Fatalf("Error getting Identity: %v \n", err)
		panic(err)
	}
	fmt.Printf("Successfully created Discogs client for user: %v\n", c.Identity.Username)
	return c
}

func generateDiscogsToken() {
	localPort = os.Getenv("LOCAL_PORT")

	config = oauth1.Config{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecretKey,
		CallbackURL:    "http://localhost:" + localPort,
		Endpoint:       discogs.Endpoint,
	}
	// Create a redirect handler
	requestToken, _, err := config.RequestToken()
	if err != nil {
		fmt.Println("Error at RequestToken:", err)
		panic(err)
	}
	authorizationUrl, err := config.AuthorizationURL(requestToken)
	if err != nil {
		fmt.Println("Error at AuthorizationURL:", err)
		panic(err)
	}

	// Create a server
	server := &http.Server{
		Addr:    ":" + localPort,
		Handler: http.HandlerFunc(handleRedirect),
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Error starting server:", err)
		}
	}()

	fmt.Printf("Authorization URL: %v\n", authorizationUrl.String())
	fmt.Println("Please verify yourself! if you are not redirected, please visit the link above")

	wg.Wait()

	// Stop the server
	if err := server.Close(); err != nil {
		fmt.Println("Error stopping server:", err)
		panic(err)
	}
}

func handleRedirect(w http.ResponseWriter, r *http.Request) {
	if handlingRedirect || doneVerifying {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("Already handling a request"))
		if err != nil {
			fmt.Println("Error writing response:", err)
		}
		return
	}
	handlingRedirect = true
	var verificationCode string
	queryParams := r.URL.Query()
	for key, values := range queryParams {
		if key == "oauth_verifier" {
			verificationCode = values[0]
		}
		for _, value := range values {
			fmt.Printf("Query parameter %s has value %s\n", key, value)
		}
	}
	// use request token to get access token
	fmt.Printf("Verification Code: %v\n", string(verificationCode))
	accessToken, accessSecret, err := config.AccessToken(requestToken, requestSecret, verificationCode)
	if err != nil {
		fmt.Println("Error at AccessToken:", err)
		handlingRedirect = false
		return
	}
	token = oauth1.NewToken(accessToken, accessSecret)

	// save token
	fmt.Printf(`
	Success! Add the following to your .zshrc or .bashrc file 
	to save your auth token as an env variable:

	// .zshrc
	export DISCOGS_TOKEN="%v"
	export DISCOGS_TOKEN_SECRET="%v"
	// .bashrc
	export DISCOGS_TOKEN="%v"
	export DISCOGS_TOKEN_SECRET="%v"
	`, token.Token, token.TokenSecret, token.Token, token.TokenSecret)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("You should be all good buddy!"))
	if err != nil {
		fmt.Println("Error writing response:", err)
	}
	handlingRedirect = false
	doneVerifying = true
}

func (c *DiscogsClient) getIdentity() error {
	path := "https://api.discogs.com/oauth/identity"

	resp, err := c.Get(path)
	if err != nil {
		fmt.Printf("Error at Get: %v \n", err)
		return err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&c.Identity)
	if err != nil {
		fmt.Printf("Error at decoding body: %v \n", err)
		return err
	}
	return nil
}

func (c *DiscogsClient) GetThumbImage(url string) (image.Image, error) {
	resp, err := c.Get(url)
	if err != nil {
		fmt.Printf("Error at Get: %v \n", err)
		return nil, err
	}
	defer resp.Body.Close()
	// Decode the body into an image.Image
	img, err := jpeg.Decode(resp.Body)
	if err != nil {
		fmt.Printf("Error decoding image: %v \n", err)
		return nil, err
	}

	return img, nil
}
