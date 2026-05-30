package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"wavelength/internal/config"
)

// Client wraps an eino ChatModel to provide LLM interactions (both streaming and non-streaming).
// The underlying eino model is created lazily on first use and cached thereafter.
type Client struct {
	cfg       *config.Config
	chatModel *openai.ChatModel
	mu        sync.Mutex
	initOnce  bool
}

// NewClient creates a new LLM client with the given configuration.
// The eino model is not created until the first call to a method that needs it.
func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// buildModel creates the underlying eino OpenAI-compatible chat model from config.
func (c *Client) buildModel(ctx context.Context) (*openai.ChatModel, error) {
	temp := float32(c.cfg.LLM.Temperature)
	if temp == 0 {
		temp = 1.0 // default for eino/openai
	}
	timeout := time.Duration(c.cfg.LLM.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	baseURL := c.cfg.LLM.Endpoint

	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL:     baseURL,
		APIKey:      c.cfg.LLM.APIKey,
		Model:       c.cfg.LLM.Model,
		Temperature: &temp,
		Timeout:     timeout,
	})
	if err != nil {
		return nil, err
	}

	return cm, nil
}

// getModel returns the cached eino model, creating it on first call.
// Thread-safe. If creation fails, subsequent calls retry.
func (c *Client) getModel(ctx context.Context) (*openai.ChatModel, error) {
	c.mu.Lock()
	if c.chatModel != nil {
		c.mu.Unlock()
		return c.chatModel, nil
	}
	if c.initOnce {
		// Creation already failed before — retry
		c.initOnce = false
	}
	cm, err := c.buildModel(ctx)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.chatModel = cm
	c.initOnce = true
	c.mu.Unlock()
	return cm, nil
}

// CheckConnectivity performs a basic connectivity check to the configured LLM endpoint.
// Sends a minimal request (max_tokens=1) to verify both reachability and credentials.
func (c *Client) CheckConnectivity(ctx context.Context) error {
	cm, err := c.getModel(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: %v", err)
	}

	_, err = cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hi"),
	}, model.WithMaxTokens(1))
	if err != nil {
		return fmt.Errorf("cannot connect to LLM service: %v", err)
	}
	return nil
}

// Call sends a non-streaming chat completion request and returns the assistant's response content.
func (c *Client) Call(ctx context.Context, messages []Message) (string, error) {
	cm, err := c.getModel(ctx)
	if err != nil {
		return "", err
	}

	msg, err := cm.Generate(ctx, c.toSchemaMessages(messages))
	if err != nil {
		log.Printf("[LLM] ERROR generating response: %v", err)
		return "", fmt.Errorf("LLM service error: %v", err)
	}

	if msg == nil {
		return "", fmt.Errorf("LLM returned empty response")
	}

	log.Printf("[LLM] Success! Response length: %d chars", len(msg.Content))
	return msg.Content, nil
}

// CallWithTools sends a chat completion request with function tools.
// It handles the tool calling loop: if the LLM requests tool calls, it executes them,
// sends results back, and continues until the LLM produces a final text response.
// Returns the assistant's conversational response.
func (c *Client) CallWithTools(ctx context.Context, messages []Message, tools []*Tool) (string, error) {
	cm, err := c.getModel(ctx)
	if err != nil {
		return "", err
	}

	// Build initial message list
	allMsgs := c.toSchemaMessages(messages)

	// Build tool lookup map
	toolMap := make(map[string]*Tool)
	for _, t := range tools {
		toolMap[t.Info.Name] = t
	}

	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Info.Name
	}
	log.Printf("[LLM-TOOLS] Starting tool-enabled call with tools: %v", toolNames)

	const maxToolRounds = 10 // Prevent infinite loops
	var totalToolCalls int

	for round := 0; round < maxToolRounds; round++ {
		// Call the LLM with tools
		callStart := time.Now()
		msg, err := cm.Generate(ctx, allMsgs, model.WithTools(ToSchemaTools(tools)))
		if err != nil {
			log.Printf("[LLM-TOOLS] ERROR generating response: %v", err)
			return "", fmt.Errorf("LLM service error: %v", err)
		}

		if msg == nil {
			return "", fmt.Errorf("LLM returned empty response")
		}

		// Check if the LLM wants to call tools
		toolCalls := ExtractToolCalls(msg)
		if len(toolCalls) == 0 {
			// No tool calls — this is the final response
			finishReason := ""
			if msg.ResponseMeta != nil {
				finishReason = msg.ResponseMeta.FinishReason
			}
			log.Printf("[LLM-TOOLS] Final response after %d round(s), %d total tool call(s), %d chars response, finish_reason=%q, %v LLM latency", round, totalToolCalls, len(msg.Content), finishReason, time.Since(callStart).Round(time.Millisecond))
			return msg.Content, nil
		}

		finishReason := ""
		if msg.ResponseMeta != nil {
			finishReason = msg.ResponseMeta.FinishReason
		}
		log.Printf("[LLM-TOOLS] Round %d: LLM requested %d tool call(s), finish_reason=%q, %v LLM latency", round, len(toolCalls), finishReason, time.Since(callStart).Round(time.Millisecond))

		// Append the assistant's message (with tool calls) to the conversation
		allMsgs = append(allMsgs, msg)

		// Execute each tool call
		for _, tc := range toolCalls {
			totalToolCalls++
			tool, ok := toolMap[tc.Name]
			if !ok {
				log.Printf("[LLM-TOOLS]  -> Unknown tool requested: %q (call id: %s)", tc.Name, tc.ID)
				allMsgs = append(allMsgs, schema.ToolMessage("error: unknown tool "+tc.Name, tc.ID))
				continue
			}

			execStart := time.Now()
			log.Printf("[LLM-TOOLS]  -> Executing tool %q (call id: %s, args: %.300q)", tc.Name, tc.ID, tc.ArgsJSON)
			result, err := tool.Execute(ctx, tc.ArgsJSON)
			elapsed := time.Since(execStart)

			if err != nil {
				log.Printf("[LLM-TOOLS]  -> Tool %q FAILED (%v): %v", tc.Name, elapsed.Round(time.Millisecond), err)
				result = fmt.Sprintf("error executing %s: %v", tc.Name, err)
			} else {
				log.Printf("[LLM-TOOLS]  -> Tool %q OK (%v, result: %d bytes)", tc.Name, elapsed.Round(time.Millisecond), len(result))
			}
			allMsgs = append(allMsgs, schema.ToolMessage(result, tc.ID))
		}
	}

	log.Printf("[LLM-TOOLS] WARNING: exceeded maximum rounds (%d) with %d total tool calls — aborting", maxToolRounds, totalToolCalls)
	return "", fmt.Errorf("tool calling exceeded maximum rounds (%d)", maxToolRounds)
}

// StreamResponse sends a streaming chat completion request and writes SSE token events
// to the provided writer. Each token chunk is emitted as a JSON event with "type": "token".
// On completion, a "type": "done" event is written. On error, a "type": "error" event is written.
func (c *Client) StreamResponse(ctx context.Context, w io.Writer, systemPrompt string, messages []Message) error {
	cm, err := c.getModel(ctx)
	if err != nil {
		return err
	}

	// Build message list: system prompt + conversation history
	allMsgs := make([]*schema.Message, 0)
	if systemPrompt != "" {
		allMsgs = append(allMsgs, schema.SystemMessage(systemPrompt))
	}
	allMsgs = append(allMsgs, c.toSchemaMessages(messages)...)

	stream, err := cm.Stream(ctx, allMsgs)
	if err != nil {
		log.Printf("[LLM-STREAM] ERROR starting stream: %v", err)
		return fmt.Errorf("LLM service error: %v", err)
	}
	defer stream.Close()

	var totalTokens, totalChars int

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[LLM-STREAM] Stream complete: %d tokens, %d chars", totalTokens, totalChars)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "done"})
			return nil
		}
		if err != nil {
			log.Printf("[LLM-STREAM] ERROR reading stream: %v", err)
			return fmt.Errorf("stream read error: %v", err)
		}

		// Skip empty content chunks (e.g., those with only tool calls)
		if chunk == nil || chunk.Content == "" {
			continue
		}

		totalTokens++
		totalChars += len(chunk.Content)

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"type":    "token",
			"content": chunk.Content,
		}); err != nil {
			log.Printf("[LLM-STREAM] ERROR writing stream event after %d tokens: %v", totalTokens, err)
			return fmt.Errorf("failed to write stream event: %v", err)
		}
	}
}

// toSchemaMessages converts the internal Message slice to eino schema.Message pointers.
func (c *Client) toSchemaMessages(messages []Message) []*schema.Message {
	msgs := make([]*schema.Message, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "system":
			msgs = append(msgs, schema.SystemMessage(m.Content))
		case "assistant":
			msgs = append(msgs, schema.AssistantMessage(m.Content, nil))
		case "user":
			msgs = append(msgs, schema.UserMessage(m.Content))
		default:
			msgs = append(msgs, schema.UserMessage(m.Content))
		}
	}
	return msgs
}

// --- Helpers kept for backward compatibility ---

// Message represents a chat message for the LLM API.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Model returns the configured LLM model name.
func (c *Client) Model() string {
	return c.cfg.LLM.Model
}

// Endpoint returns the configured LLM endpoint base URL.
func (c *Client) Endpoint() string {
	return c.cfg.LLM.Endpoint
}

// PersonaPrompt returns the configured persona system prompt.
func (c *Client) PersonaPrompt() string {
	return c.cfg.GetPersonaPrompt()
}

// Timeout returns the configured HTTP timeout in seconds (default 60).
func (c *Client) Timeout() int {
	if c.cfg.LLM.Timeout > 0 {
		return c.cfg.LLM.Timeout
	}
	return 60
}

// APIKey returns the configured LLM API key.
func (c *Client) APIKey() string {
	return c.cfg.LLM.APIKey
}

// APIPath returns the configured API path (default "/chat/completions").
func (c *Client) APIPath() string {
	if c.cfg.LLM.Path != "" {
		return c.cfg.LLM.Path
	}
	return "/chat/completions"
}

// APIURL returns the full URL for chat completions (endpoint + path).
// Note: eino internally handles path appending, but this method is kept
// for reference/logging purposes.
func (c *Client) APIURL() string {
	return c.cfg.LLM.Endpoint + c.APIPath()
}

// Transcribe sends audio data to the configured transcription endpoint and returns the transcribed text.
// Supports two server types:
//   - "openai" (default): OpenAI-compatible /v1/audio/transcriptions endpoint with Bearer auth
//   - "whispercpp": whisper.cpp /inference endpoint with no auth and array-style JSON response
func (c *Client) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	endpoint := c.cfg.Voice.WhisperURL
	if endpoint == "" {
		endpoint = c.cfg.LLM.Endpoint
	}
	whisperType := c.cfg.Voice.WhisperType
	if whisperType == "" {
		whisperType = "openai"
	}

	timeout := time.Duration(c.cfg.LLM.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	switch whisperType {
	case "whispercpp":
		return c.transcribeWhisperCPP(ctx, endpoint, audioData, timeout)
	default:
		return c.transcribeOpenAI(ctx, endpoint, audioData, timeout)
	}
}

// transcribeOpenAI sends audio to an OpenAI-compatible /v1/audio/transcriptions endpoint.
func (c *Client) transcribeOpenAI(ctx context.Context, endpoint string, audioData []byte, timeout time.Duration) (string, error) {
	model := c.cfg.Voice.WhisperModel
	if model == "" {
		model = "whisper-1"
	}

	// Create a temporary file for the multipart upload
	tmpFile, err := os.CreateTemp("", "voice-*.webm")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(audioData); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write audio data: %v", err)
	}
	tmpFile.Close()

	// Build multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file
	fileWriter, err := writer.CreateFormFile("file", filepath.Base(tmpFile.Name()))
	if err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	file, err := os.Open(tmpFile.Name())
	if err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to open temp file: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(fileWriter, file); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to copy file data: %v", err)
	}

	// Add the model field
	if err := writer.WriteField("model", model); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write model field: %v", err)
	}

	writer.Close()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.cfg.LLM.APIKey)

	// Execute request
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse transcription response: %v", err)
	}

	log.Printf("[VOICE] Transcription successful (openai): %d chars", len(result.Text))
	return result.Text, nil
}

// transcribeWhisperCPP sends audio to a whisper.cpp /inference endpoint.
// Converts the audio to WAV format first, as whisper.cpp requires WAV input.
func (c *Client) transcribeWhisperCPP(ctx context.Context, endpoint string, audioData []byte, timeout time.Duration) (string, error) {
	var wavFile string
	var tmpFile *os.File
	var err error

	// Check if the audio is already a WAV (from CheckWhisper test)
	if isWAV(audioData) {
		// Already WAV, use it directly
		tmpFile, err = os.CreateTemp("", "voice-*.wav")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(audioData); err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to write audio data: %v", err)
		}
		tmpFile.Close()
		wavFile = tmpFile.Name()
	} else {
		// Convert to WAV using ffmpeg (whisper.cpp requires WAV)
		tmpFile, err = os.CreateTemp("", "voice-*.webm")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(audioData); err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to write audio data: %v", err)
		}
		tmpFile.Close()

		wavFile, err = convertToWAV(ctx, tmpFile.Name())
		if err != nil {
			return "", fmt.Errorf("failed to convert audio to WAV: %v", err)
		}
		defer os.Remove(wavFile)
	}

	// Build multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the WAV file
	fileWriter, err := writer.CreateFormFile("file", filepath.Base(wavFile))
	if err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to create form file: %v", err)
	}

	file, err := os.Open(wavFile)
	if err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to open WAV file: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(fileWriter, file); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to copy file data: %v", err)
	}

	// Add response_format field
	if err := writer.WriteField("response_format", "json"); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write response_format field: %v", err)
	}

	writer.Close()

	// Create HTTP request — whisper.cpp uses /inference, no auth
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/inference", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("transcription failed (HTTP %d)", resp.StatusCode)
	}

	// Parse response — whisper.cpp returns {"text": "..."} or {"text": ["...", ...]}
	var rawResult map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &rawResult); err != nil {
		return "", fmt.Errorf("failed to parse transcription response: %v", err)
	}

	var text string
	if rawText, ok := rawResult["text"]; ok {
		// Try array first, then fall back to string
		var arr []string
		if err := json.Unmarshal(rawText, &arr); err == nil {
			text = strings.Join(arr, "")
		} else {
			if err := json.Unmarshal(rawText, &text); err != nil {
				return "", fmt.Errorf("failed to parse transcription text: %v", err)
			}
		}
	}

	log.Printf("[VOICE] Transcription successful (whispercpp): %d chars", len(text))
	return text, nil
}

// isWAV checks if the audio data is a WAV file by checking the RIFF header.
func isWAV(data []byte) bool {
	return len(data) >= 4 && string(data[0:4]) == "RIFF"
}

// convertToWAV converts an audio file to WAV format using ffmpeg.
// whisper.cpp requires WAV input, so this converts WebM/Opus from the browser.
func convertToWAV(ctx context.Context, inputFile string) (string, error) {
	// Create a temporary WAV file
	wavFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create temp WAV file: %v", err)
	}
	wavPath := wavFile.Name()
	wavFile.Close()

	// Run ffmpeg to convert to WAV (16kHz, mono, 16-bit PCM)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inputFile, "-ar", "16000", "-ac", "1", "-f", "wav", wavPath)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		os.Remove(wavPath)
		return "", fmt.Errorf("ffmpeg conversion failed: %v", err)
	}

	return wavPath, nil
}

// CheckWhisper verifies that the LLM endpoint supports audio transcription.
// Sends a minimal 1-second beep WAV to test the endpoint.
func (c *Client) CheckWhisper(ctx context.Context) error {
	// Generate a minimal valid WAV file with a beep tone (1 second, 16kHz, mono, 16-bit)
	wavData := generateBeepWAV(16000, 1, 440)

	_, err := c.Transcribe(ctx, wavData)
	// The transcription may return empty text for silence, which is fine.
	// We only care if the endpoint exists and responds with valid JSON.
	if err != nil {
		return fmt.Errorf("whisper endpoint check failed: %v", err)
	}
	return nil
}

// generateBeepWAV creates a minimal valid WAV file with a sine wave beep tone using go-audio/wav.
// sampleRate: samples per second (e.g., 16000)
// durationSec: duration in seconds
// frequency: frequency of the beep in Hz (e.g., 440)
func generateBeepWAV(sampleRate int, durationSec int, frequency int) []byte {
	numSamples := sampleRate * durationSec

	// Generate sine wave samples as int16
	int16Samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		phase := float64(i) * 2 * math.Pi * float64(frequency) / float64(sampleRate)
		int16Samples[i] = int16(16384 * math.Sin(phase)) // ~50% amplitude
	}

	// Convert to audio.IntBuffer (go-audio uses int, not int16)
	intSamples := make([]int, numSamples)
	for i, s := range int16Samples {
		intSamples[i] = int(s)
	}

	audioBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  sampleRate,
		},
		Data:           intSamples,
		SourceBitDepth: 16,
	}

	// Create a temp file for the encoder (needs io.WriteSeeker)
	tmpFile, err := os.CreateTemp("", "voice-check-*.wav")
	if err != nil {
		log.Printf("[VOICE] failed to create temp WAV file: %v", err)
		return nil
	}
	defer os.Remove(tmpFile.Name())

	// 1 = PCM (linear) audio format
	enc := wav.NewEncoder(tmpFile, sampleRate, 16, 1, 1)
	if err := enc.Write(audioBuf); err != nil {
		log.Printf("[VOICE] failed to encode WAV: %v", err)
		tmpFile.Close()
		return nil
	}
	enc.Close()
	tmpFile.Close()

	// Read the WAV file back
	wavData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		log.Printf("[VOICE] failed to read WAV file: %v", err)
		return nil
	}

	return wavData
}
