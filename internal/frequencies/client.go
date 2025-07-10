package frequencies

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yegors/co-atc/pkg/logger"
)

// Client is responsible for fetching audio streams from audio sources
type Client struct {
	httpClient *http.Client
	logger     *logger.Logger
}

// NewClient creates a new client for fetching audio streams
func NewClient(timeout time.Duration, logger *logger.Logger) *Client {
	// Create a transport with keep-alive enabled
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Disable compression for audio streams
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second, // Keep-alive period
		}).DialContext,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		logger: logger.Named("frequencies-client"),
	}
}

// addCacheBreaker adds a dynamic cache breaker to the URL
func (c *Client) addCacheBreaker(url string) string {
	// Generate a timestamp-based cache breaker similar to the example
	timestamp := time.Now().UnixNano()
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%snocache=%d", url, separator, timestamp)
}

// StreamAudio starts streaming audio from the source
func (c *Client) StreamAudio(ctx context.Context, opts StreamOptions) (io.ReadCloser, http.Header, error) {
	// Add cache breaker to URL
	urlWithCacheBreaker := c.addCacheBreaker(opts.URL)

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlWithCacheBreaker, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set essential headers for HTTP audio streams
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive") // Keep connection alive for streaming
	req.Header.Set("User-Agent", "Co-ATC/1.0") // Generic user agent

	// Execute the request
	c.logger.Debug("Connecting to audio stream",
		logger.String("url", urlWithCacheBreaker),
	)

	// Create a retry mechanism
	maxRetries := 3
	retryDelay := 1 * time.Second

	var resp *http.Response

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break // Success, exit retry loop
		}

		// Close the response body if there was an error
		if resp != nil {
			resp.Body.Close()
		}

		// If this was the last attempt, return the error
		if attempt == maxRetries-1 {
			if err != nil {
				return nil, nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries, err)
			}
			return nil, nil, fmt.Errorf("unexpected status code after %d attempts: %d", maxRetries, resp.StatusCode)
		}

		// Log the retry
		c.logger.Warn("Retrying connection to audio stream",
			logger.String("url", urlWithCacheBreaker),
			logger.Int("attempt", attempt+1),
			logger.Int("max_attempts", maxRetries),
			logger.Error(err),
		)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(retryDelay):
			// Exponential backoff
			retryDelay *= 2
		}
	}

	c.logger.Debug("Successfully connected to audio stream",
		logger.String("url", urlWithCacheBreaker),
		logger.String("content_type", resp.Header.Get("Content-Type")),
	)

	// Create a buffered reader to improve performance
	bufferedReader := bufio.NewReaderSize(resp.Body, 64*1024) // 64KB buffer

	// Create a readCloser that closes the original response body
	readCloser := &bufferedReadCloser{
		Reader: bufferedReader,
		Closer: resp.Body,
	}

	return readCloser, resp.Header, nil
}

// bufferedReadCloser combines a buffered reader with a closer
type bufferedReadCloser struct {
	Reader *bufio.Reader
	Closer io.Closer
}

// Read implements the io.Reader interface
func (b *bufferedReadCloser) Read(p []byte) (n int, err error) {
	return b.Reader.Read(p)
}

// Close implements the io.Closer interface
func (b *bufferedReadCloser) Close() error {
	return b.Closer.Close()
}

// ExtractMetadata extracts metadata from response headers
func (c *Client) ExtractMetadata(headers http.Header) StreamMetadata {
	metadata := StreamMetadata{
		ContentType: headers.Get("Content-Type"),
		Description: headers.Get("icy-description"),
		Genre:       headers.Get("icy-genre"),
		Name:        headers.Get("icy-name"),
	}

	// Extract bitrate
	if bitrateStr := headers.Get("icy-br"); bitrateStr != "" {
		var bitrate int
		if _, err := fmt.Sscanf(bitrateStr, "%d", &bitrate); err == nil {
			metadata.Bitrate = bitrate
		}
	}

	// Set format based on content type
	switch metadata.ContentType {
	case "audio/mpeg":
		metadata.Format = "mp3"
	case "audio/aac":
		metadata.Format = "aac"
	case "audio/ogg":
		metadata.Format = "ogg"
	default:
		metadata.Format = "unknown"
	}

	return metadata
}
