package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// setupSearchTest creates a mock Notion API server for search-related tests.
func setupSearchTest(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret_valid_token" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "unauthorized",
				"message": "API token is invalid.",
			})
			return
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		query, _ := body["query"].(string)
		startCursor, _ := body["start_cursor"].(string)

		// Pagination: page 2
		if startCursor == "cursor_page2" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					mockPage("page-3", "Third Page", "2026-03-15T10:00:00.000Z"),
				},
				"has_more":    false,
				"next_cursor": nil,
			})
			return
		}

		// Filter handling
		if filter, ok := body["filter"].(map[string]interface{}); ok {
			val, _ := filter["value"].(string)
			if val == "page" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						mockPage("page-1", "Meeting Notes", "2026-03-20T10:00:00.000Z"),
					},
					"has_more":    false,
					"next_cursor": nil,
				})
				return
			}
			if val == "database" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						mockDatabase("db-1", "Task Tracker", "2026-03-19T08:00:00.000Z"),
					},
					"has_more":    false,
					"next_cursor": nil,
				})
				return
			}
		}

		// Empty query → all results
		if query == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					mockPage("page-1", "Meeting Notes", "2026-03-20T10:00:00.000Z"),
					mockDatabase("db-1", "Task Tracker", "2026-03-19T08:00:00.000Z"),
				},
				"has_more":    false,
				"next_cursor": nil,
			})
			return
		}

		// Query with pagination
		if query == "paginated" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					mockPage("page-1", "Paginated One", "2026-03-20T10:00:00.000Z"),
					mockPage("page-2", "Paginated Two", "2026-03-19T10:00:00.000Z"),
				},
				"has_more":    true,
				"next_cursor": "cursor_page2",
			})
			return
		}

		// No results
		if query == "nonexistent_xyz_nothing" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results":     []interface{}{},
				"has_more":    false,
				"next_cursor": nil,
			})
			return
		}

		// Default: return matching results
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{
				mockPage("page-1", "Meeting Notes", "2026-03-20T10:00:00.000Z"),
			},
			"has_more":    false,
			"next_cursor": nil,
		})
	})

	server := httptest.NewServer(mux)

	t.Setenv("NOTION_BASE_URL", server.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("NOTION_TOKEN", "secret_valid_token")

	return server
}

func mockPage(id, title, lastEdited string) map[string]interface{} {
	return map[string]interface{}{
		"object":           "page",
		"id":               id,
		"last_edited_time": lastEdited,
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type": "title",
				"title": []interface{}{
					map[string]interface{}{
						"plain_text": title,
					},
				},
			},
		},
	}
}

func mockDatabase(id, title, lastEdited string) map[string]interface{} {
	return map[string]interface{}{
		"object":           "database",
		"id":               id,
		"last_edited_time": lastEdited,
		"title": []interface{}{
			map[string]interface{}{
				"plain_text": title,
			},
		},
	}
}

// captureStdout runs fn while capturing os.Stdout output.
// render.Table and fmt.Println write to os.Stdout, not cobra's buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// runSearch executes a search command and returns combined output (cobra + stdout).
// Resets global cobra state (outputFormat, debugMode) and search-specific flags.
func runSearch(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Reset global flags that persist across tests
	outputFormat = ""
	debugMode = false
	// Reset search command local flags to defaults
	_ = searchCmd.Flags().Set("type", "")
	_ = searchCmd.Flags().Set("limit", "10")
	_ = searchCmd.Flags().Set("cursor", "")
	_ = searchCmd.Flags().Set("all", "false")
	var cobraOut string
	var err error
	captured := captureStdout(t, func() {
		cobraOut, _, err = executeCommand(args...)
	})
	return cobraOut + captured, err
}

// --- Basic search ---

func TestSearchBasicQuery(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "meeting")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Meeting Notes") {
		t.Errorf("expected 'Meeting Notes' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "page") {
		t.Errorf("expected 'page' type in output, got: %s", stdout)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Meeting Notes") {
		t.Errorf("expected 'Meeting Notes' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Task Tracker") {
		t.Errorf("expected 'Task Tracker' in output, got: %s", stdout)
	}
}

func TestSearchNoResults(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "nonexistent_xyz_nothing")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "No results found") {
		t.Errorf("expected 'No results found' in output, got: %s", stdout)
	}
}

// --- Type filter ---

func TestSearchFilterPage(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "--type", "page", "meeting")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Meeting Notes") {
		t.Errorf("expected 'Meeting Notes' in output, got: %s", stdout)
	}
	if strings.Contains(stdout, "Task Tracker") {
		t.Errorf("should not contain 'Task Tracker' when filtering by page")
	}
}

func TestSearchFilterDatabase(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "--type", "database", "tracker")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Task Tracker") {
		t.Errorf("expected 'Task Tracker' in output, got: %s", stdout)
	}
	if strings.Contains(stdout, "Meeting Notes") {
		t.Errorf("should not contain 'Meeting Notes' when filtering by database")
	}
}

// --- JSON output ---

func TestSearchJSONOutput(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "meeting", "--format", "json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid JSON, got parse error: %v\noutput: %s", err, stdout)
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Fatal("expected 'results' array in JSON output")
	}
	if len(results) == 0 {
		t.Error("expected at least one result in JSON output")
	}
}

// --- Pagination ---

func TestSearchPagination(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "--all", "paginated")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Paginated One") {
		t.Errorf("expected 'Paginated One' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Paginated Two") {
		t.Errorf("expected 'Paginated Two' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Third Page") {
		t.Errorf("expected 'Third Page' (page 2) in output, got: %s", stdout)
	}
}

func TestSearchPaginationJSON(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "--all", "--format", "json", "paginated")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %v\noutput: %s", err, stdout)
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Fatal("expected 'results' array in JSON output")
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results from 2 pages, got %d", len(results))
	}
}

func TestSearchWithCursor(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "--cursor", "cursor_page2", "anything")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Third Page") {
		t.Errorf("expected 'Third Page' in output, got: %s", stdout)
	}
}

// --- Limit flag ---

func TestSearchWithLimit(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	_, err := runSearch(t, "search", "--limit", "5", "meeting")
	if err != nil {
		t.Fatalf("expected no error with --limit, got: %v", err)
	}
}

// --- Auth errors ---

func TestSearchUnauthorized(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	t.Setenv("NOTION_TOKEN", "bad_token")

	_, err := runSearch(t, "search", "test")
	if err == nil {
		t.Fatal("expected error for unauthorized token")
	}
}

// --- Multi-word query ---

func TestSearchMultiWordQuery(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search", "meeting", "notes")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "Meeting Notes") {
		t.Errorf("expected 'Meeting Notes' in output, got: %s", stdout)
	}
}

// --- Table output format ---

func TestSearchTableOutputContainsData(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Table should contain data rows (headers may go through color.Printf which
	// bypasses os.Stdout pipe, but data rows use fmt.Printf)
	if !strings.Contains(stdout, "page-1") {
		t.Errorf("expected 'page-1' ID in table output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "db-1") {
		t.Errorf("expected 'db-1' ID in table output, got: %s", stdout)
	}
	// Separator line
	if !strings.Contains(stdout, "───") {
		t.Errorf("expected separator line in table output, got: %s", stdout)
	}
}

// --- Database icon ---

func TestSearchDatabaseIcon(t *testing.T) {
	server := setupSearchTest(t)
	defer server.Close()

	stdout, err := runSearch(t, "search")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(stdout, "🗃️") {
		t.Errorf("expected database icon '🗃️' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "📄") {
		t.Errorf("expected page icon '📄' in output, got: %s", stdout)
	}
}
