package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/config"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

// testConfig returns a default LLM config for tests.
// Override any field as needed.
func testConfig(t *testing.T) config.LLMConfig {
	t.Helper()
	return config.LLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
		Endpoint: "http://localhost:11434",
		APIKey:   "test-key",
	}
}

// testApp creates a fresh Fiber app with routes, a new in-memory topic store,
// and an LLM client configured with default test values.
func testApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	store := topic.NewStore()
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 3000},
		LLM:     testConfig(t),
		DataDir: t.TempDir(),
	}
	client := llm.NewClient(cfg)
	SetupRoutes(app, store, client)
	return app
}

// TestSuite holds the dependencies for an API integration test.
// Use [newSuite] or [newSuiteWithMock] to create one.
type TestSuite struct {
	App    *fiber.App
	Store  *topic.Store
	Client *llm.Client
	Config *config.Config
	Dir    string
}

// newSuite creates a TestSuite with a default config (no mock LLM server).
// The store is in-memory (topic.Store), the data dir is a temp directory.
func newSuite(t *testing.T) *TestSuite {
	t.Helper()
	store := topic.NewStore()
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 3000},
		LLM:     testConfig(t),
		DataDir: t.TempDir(),
	}
	client := llm.NewClient(cfg)
	app := fiber.New()
	SetupRoutes(app, store, client)
	return &TestSuite{App: app, Store: store, Client: client, Config: cfg, Dir: t.TempDir()}
}

// SuiteWithMock is like TestSuite but includes a mock LLM server and the body
// captured from the last LLM request.
type SuiteWithMock struct {
	*TestSuite
	MockServer      *httptest.Server
	LastRequestBody string
}

// newSuiteWithMock creates a TestSuite wired to an httptest server.
// The handler receives each LLM request and can inspect the body.
// After the test, call suite.MockServer.Close() or use defer suite.Cleanup(t).
func newSuiteWithMock(t *testing.T, handler http.HandlerFunc) *SuiteWithMock {
	t.Helper()
	server := httptest.NewServer(handler)
	endpoint := server.URL
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 3000},
		LLM:     config.LLMConfig{Provider: "openai", Model: "gpt-4", Endpoint: endpoint, APIKey: "test-key"},
		DataDir: t.TempDir(),
	}
	client := llm.NewClient(cfg)
	store := topic.NewStore()
	app := fiber.New()
	SetupRoutes(app, store, client)
	return &SuiteWithMock{
		TestSuite:  &TestSuite{App: app, Store: store, Client: client, Config: cfg, Dir: t.TempDir()},
		MockServer: server,
	}
}

// newSuiteWithMockConfig creates a TestSuite wired to an httptest server with
// a custom LLM config (e.g., different provider or model).
func newSuiteWithMockConfig(t *testing.T, handler http.HandlerFunc, llmCfg config.LLMConfig) *SuiteWithMock {
	t.Helper()
	server := httptest.NewServer(handler)
	llmCfg.Endpoint = server.URL
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 3000},
		LLM:     llmCfg,
		DataDir: t.TempDir(),
	}
	client := llm.NewClient(cfg)
	store := topic.NewStore()
	app := fiber.New()
	SetupRoutes(app, store, client)
	return &SuiteWithMock{
		TestSuite:  &TestSuite{App: app, Store: store, Client: client, Config: cfg, Dir: t.TempDir()},
		MockServer: server,
	}
}

// Cleanup closes the mock server.
func (s *SuiteWithMock) Cleanup(t *testing.T) {
	t.Helper()
	s.MockServer.Close()
}

// --- Request helpers ---

// PostJSON sends a POST request to the given path with a JSON body.
// Returns the HTTP response body as []byte and the status code.
func (s *TestSuite) PostJSON(t *testing.T, path string, body map[string]string) ([]byte, int) {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode
}

// PostJSONBytes sends a POST request with raw JSON bytes.
func (s *TestSuite) PostJSONBytes(t *testing.T, path string, data []byte) ([]byte, int) {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode
}

// Get sends a GET request.
func (s *TestSuite) Get(t *testing.T, path string) ([]byte, int) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode
}

// --- Response helpers ---

// AssertStatus checks that the response has the expected HTTP status code.
func AssertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("expected status %d, got %d", want, got)
	}
}

// ErrorResponse is the shape of a JSON error response from the API.
type ErrorResponse struct {
	Message string `json:"message"`
}

// MustDecodeJSON decodes a JSON response body into v, failing the test on error.
func MustDecodeJSON[T any](t *testing.T, body []byte, v *T) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("expected JSON response, got: %v (body: %s)", err, string(body))
	}
}

// CreateTopic creates a topic via the API and returns its ID.
// Useful when tests need to create topics without duplicating the JSON payload.
func CreateTopic(t *testing.T, app *fiber.App, name, desc string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"name":"%s","description":"%s"}`, name, desc)
	req := httptest.NewRequest("POST", "/api/topics", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	id, ok := result["id"].(string)
	if !ok || id == "" {
		t.Fatalf("no id in response: %s", string(body))
	}
	return id
}

// AssertErrorContains asserts that the JSON error response message contains the given substring.
func AssertErrorContains(t *testing.T, body []byte, substr string) {
	t.Helper()
	var errResp ErrorResponse
	MustDecodeJSON(t, body, &errResp)
	if !strings.Contains(strings.ToLower(errResp.Message), strings.ToLower(substr)) {
		t.Errorf("expected error message to contain %q, got: %q", substr, errResp.Message)
	}
}
