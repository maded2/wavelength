package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// E1-S3: User accesses the web application from a browser

func TestLandingPage(t *testing.T) {
	app := fiber.New()
	app.Get("/", LandingPage)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		contentType    string
		bodyContains   []string
	}{
		{
			name:           "displays the Wavelength interface on root path",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			contentType:    "text/html",
			bodyContains:   []string{"Wavelength"},
		},
		{
			name:           "responds with HTML content type",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			contentType:    "text/html",
			bodyContains:   []string{"<!DOCTYPE html>", "<html"},
		},
		{
			name:           "interface uses plain language understandable by non-technical users",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			contentType:    "text/html",
			bodyContains:   []string{"requirement"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, tc.contentType) {
				t.Errorf("expected content type to contain %s, got %s", tc.contentType, contentType)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			bodyStr := string(body)
			for _, expected := range tc.bodyContains {
				if !strings.Contains(bodyStr, expected) {
					t.Errorf("expected body to contain %q, got: %s", expected, bodyStr)
				}
			}
		})
	}
}
