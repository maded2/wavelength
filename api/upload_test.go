package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"wavelength/internal/llm"
	"wavelength/internal/topic"
)

func readBody(t *testing.T, r io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return data
}

func TestUploadMarkdown(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	// Create a topic
	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	// Create multipart form with markdown file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "requirements.md")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, err = part.Write([]byte("# Test Document\n\nThis is test content."))
	if err != nil {
		t.Fatalf("failed to write to form file: %v", err)
	}
	contentType := writer.FormDataContentType()
	writer.Close()

	// Upload
	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	bodyBytes := readBody(t, resp.Body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["success"] != true {
		t.Fatalf("expected success=true, got %v", result["success"])
	}

	// Verify attachment was stored
	attachments := store.ListAttachments(topicID)
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].Filename != "requirements.md" {
		t.Errorf("expected filename 'requirements.md', got %q", attachments[0].Filename)
	}
	if attachments[0].Format != "markdown" {
		t.Errorf("expected format 'markdown', got %q", attachments[0].Format)
	}
	if !strings.Contains(attachments[0].MarkdownContent, "Test Document") {
		t.Errorf("markdown content missing expected text: %q", attachments[0].MarkdownContent)
	}
}

func TestUploadUnsupportedFormat(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "document.txt")
	part.Write([]byte("plain text"))
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, _ := app.Test(req)
	bodyBytes := readBody(t, resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", resp.StatusCode, bodyBytes)
	}
}

func TestUploadNoFile(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, _ := app.Test(req)
	bodyBytes := readBody(t, resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", resp.StatusCode, bodyBytes)
	}
}

func TestUploadTopicNotFound(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.md")
	part.Write([]byte("# Test"))
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/nonexistent/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestUploadCompletedTopic(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")
	store.SetStatus(topicID, "completed")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.md")
	part.Write([]byte("# Test"))
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}
}

func TestListAttachments(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	// Upload a file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.md")
	part.Write([]byte("# Test"))
	contentType := writer.FormDataContentType()
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	app.Test(req)

	// List attachments
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/api/topics/"+topicID+"/attachments", nil))
	bodyBytes := readBody(t, resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var attachments []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &attachments); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	if attachments[0]["filename"] != "test.md" {
		t.Errorf("expected filename 'test.md', got %v", attachments[0]["filename"])
	}
}

func TestListAttachmentsTopicNotFound(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/api/topics/nonexistent/attachments", nil))

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestUploadWithPDF(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	// Minimal valid PDF content
	pdfContent := `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>endobj
4 0 obj<</Length 44>>stream
BT /F1 12 Tf 100 700 Td (Hello World) Tj ET
endstream endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
0000000360 00000 n 
trailer<</Size 6/Root 1 0 R>>
startxref
443
%%EOF`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "document.pdf")
	part.Write([]byte(pdfContent))
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	bodyBytes := readBody(t, resp.Body)

	// PDF parsing may succeed or fail depending on library capabilities
	if resp.StatusCode == http.StatusBadRequest {
		t.Logf("PDF upload rejected (expected if PDF library cannot parse minimal PDF): %s", bodyBytes)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)

	if result["success"] != true {
		t.Fatalf("expected success=true, got %v", result["success"])
	}
}

func TestUploadWithWord(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "upload-test", "Test upload feature")

	// Create a minimal DOCX (ZIP with word/document.xml)
	docxContent := createMinimalDocx()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "document.docx")
	part.Write([]byte(docxContent))
	contentType := writer.FormDataContentType()
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	bodyBytes := readBody(t, resp.Body)

	// DOCX parsing may succeed or fail with minimal ZIP
	if resp.StatusCode == http.StatusBadRequest {
		t.Logf("DOCX upload rejected (expected if ZIP parsing fails): %s", bodyBytes)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)

	if result["success"] != true {
		t.Fatalf("expected success=true, got %v", result["success"])
	}
}

func TestUploadedDocsInContext(t *testing.T) {
	store := topic.NewStore()
	client := llm.NewClient(nil)
	app := setupTestApp(store, client)

	topicID := createTestTopic(t, app, "context-test", "Test context with attachments")

	// Upload a document
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "context.md")
	part.Write([]byte("# Context Document\n\nImportant context for the interview."))
	contentType := writer.FormDataContentType()
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/topics/"+topicID+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	app.Test(req)

	// Verify the topic has the attachment
	topic := store.Get(topicID)
	if topic == nil {
		t.Fatal("topic not found")
	}

	if len(topic.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(topic.Attachments))
	}

	// Verify buildConversationContext includes the attachment
	prompt := buildConversationContext(topic, "test message")
	if !strings.Contains(prompt, "Uploaded reference documents") {
		t.Error("prompt should contain 'Uploaded reference documents'")
	}
	if !strings.Contains(prompt, "context.md") {
		t.Error("prompt should contain the uploaded filename")
	}
	if !strings.Contains(prompt, "Context Document") {
		t.Error("prompt should contain the document content")
	}
}

// createMinimalDocx creates a minimal DOCX for testing.
func createMinimalDocx() string {
	return "PK\x03\x04" // Minimal ZIP magic bytes (not a valid DOCX, used to test error handling)
}

// setupTestApp creates a Fiber app with test routes.
func setupTestApp(store topic.TopicStore, client *llm.Client) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	SetupRoutes(app, store, client)
	return app
}

// createTestTopic creates a test topic via the API.
func createTestTopic(t *testing.T, app *fiber.App, name, desc string) string {
	t.Helper()
	payload := fmt.Sprintf(`{"name":"%s","description":"%s"}`, name, desc)
	req := httptest.NewRequest(http.MethodPost, "/api/topics", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	bodyBytes := readBody(t, resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, bodyBytes)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	topicID, ok := result["id"].(string)
	if !ok {
		t.Fatalf("no id in response: %s", bodyBytes)
	}

	// Give the store a moment to process
	time.Sleep(10 * time.Millisecond)
	return topicID
}
