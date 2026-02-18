package cmd

import (
	"testing"
)

func TestParseFilter(t *testing.T) {
	// Mock database properties schema
	dbProps := map[string]interface{}{
		"Status": map[string]interface{}{
			"type": "select",
		},
		"Name": map[string]interface{}{
			"type": "title",
		},
		"Date": map[string]interface{}{
			"type": "date",
		},
		"Count": map[string]interface{}{
			"type": "number",
		},
		"Tags": map[string]interface{}{
			"type": "multi_select",
		},
		"Done": map[string]interface{}{
			"type": "checkbox",
		},
		"Priority": map[string]interface{}{
			"type": "status",
		},
		"Website": map[string]interface{}{
			"type": "url",
		},
	}

	tests := []struct {
		name    string
		expr    string
		wantErr bool
		check   func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "select equals",
			expr: "Status=Done",
			check: func(t *testing.T, r map[string]interface{}) {
				if r["property"] != "Status" {
					t.Errorf("property = %v, want Status", r["property"])
				}
				sel, ok := r["select"].(map[string]interface{})
				if !ok {
					t.Fatal("expected select filter")
				}
				if sel["equals"] != "Done" {
					t.Errorf("select.equals = %v, want Done", sel["equals"])
				}
			},
		},
		{
			name: "select not equals",
			expr: "Status!=Cancelled",
			check: func(t *testing.T, r map[string]interface{}) {
				sel := r["select"].(map[string]interface{})
				if sel["does_not_equal"] != "Cancelled" {
					t.Errorf("select.does_not_equal = %v, want Cancelled", sel["does_not_equal"])
				}
			},
		},
		{
			name: "title contains",
			expr: "Name~=meeting",
			check: func(t *testing.T, r map[string]interface{}) {
				title := r["title"].(map[string]interface{})
				if title["contains"] != "meeting" {
					t.Errorf("title.contains = %v, want meeting", title["contains"])
				}
			},
		},
		{
			name: "date greater than or equal",
			expr: "Date>=2026-01-01",
			check: func(t *testing.T, r map[string]interface{}) {
				date := r["date"].(map[string]interface{})
				if date["on_or_after"] != "2026-01-01" {
					t.Errorf("date.on_or_after = %v, want 2026-01-01", date["on_or_after"])
				}
			},
		},
		{
			name: "date less than or equal",
			expr: "Date<=2026-12-31",
			check: func(t *testing.T, r map[string]interface{}) {
				date := r["date"].(map[string]interface{})
				if date["on_or_before"] != "2026-12-31" {
					t.Errorf("date.on_or_before = %v, want 2026-12-31", date["on_or_before"])
				}
			},
		},
		{
			name: "number greater than",
			expr: "Count>10",
			check: func(t *testing.T, r map[string]interface{}) {
				num := r["number"].(map[string]interface{})
				if num["greater_than"] != float64(10) {
					t.Errorf("number.greater_than = %v, want 10", num["greater_than"])
				}
			},
		},
		{
			name: "number equals",
			expr: "Count=42",
			check: func(t *testing.T, r map[string]interface{}) {
				num := r["number"].(map[string]interface{})
				if num["equals"] != float64(42) {
					t.Errorf("number.equals = %v, want 42", num["equals"])
				}
			},
		},
		{
			name: "multi_select contains",
			expr: "Tags~=urgent",
			check: func(t *testing.T, r map[string]interface{}) {
				ms := r["multi_select"].(map[string]interface{})
				if ms["contains"] != "urgent" {
					t.Errorf("multi_select.contains = %v, want urgent", ms["contains"])
				}
			},
		},
		{
			name: "status equals",
			expr: "Priority=High",
			check: func(t *testing.T, r map[string]interface{}) {
				s := r["status"].(map[string]interface{})
				if s["equals"] != "High" {
					t.Errorf("status.equals = %v, want High", s["equals"])
				}
			},
		},
		{
			name: "checkbox",
			expr: "Done=true",
			check: func(t *testing.T, r map[string]interface{}) {
				cb := r["checkbox"].(map[string]interface{})
				if cb["equals"] != true {
					t.Errorf("checkbox.equals = %v, want true", cb["equals"])
				}
			},
		},
		{
			name:    "unknown property",
			expr:    "NonExistent=value",
			wantErr: true,
		},
		{
			name:    "no operator",
			expr:    "StatusDone",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFilter(tt.expr, dbProps)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		wantProp  string
		wantDir   string
	}{
		{
			name:     "descending",
			expr:     "Date:desc",
			wantProp: "Date",
			wantDir:  "descending",
		},
		{
			name:     "ascending explicit",
			expr:     "Name:asc",
			wantProp: "Name",
			wantDir:  "ascending",
		},
		{
			name:     "ascending default",
			expr:     "Name",
			wantProp: "Name",
			wantDir:  "ascending",
		},
		{
			name:     "descending full word",
			expr:     "Date:descending",
			wantProp: "Date",
			wantDir:  "descending",
		},
		{
			name:     "with spaces",
			expr:     " Date : desc ",
			wantProp: "Date",
			wantDir:  "descending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSort(tt.expr)
			if result["property"] != tt.wantProp {
				t.Errorf("property = %v, want %v", result["property"], tt.wantProp)
			}
			if result["direction"] != tt.wantDir {
				t.Errorf("direction = %v, want %v", result["direction"], tt.wantDir)
			}
		})
	}
}

func TestMapTextOp(t *testing.T) {
	tests := []struct{ op, want string }{
		{"eq", "equals"},
		{"neq", "does_not_equal"},
		{"contains", "contains"},
		{"not_contains", "does_not_contain"},
		{"unknown", "equals"},
	}
	for _, tt := range tests {
		if got := mapTextOp(tt.op); got != tt.want {
			t.Errorf("mapTextOp(%q) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestMapNumberOp(t *testing.T) {
	tests := []struct{ op, want string }{
		{"eq", "equals"},
		{"neq", "does_not_equal"},
		{"gt", "greater_than"},
		{"gte", "greater_than_or_equal_to"},
		{"lt", "less_than"},
		{"lte", "less_than_or_equal_to"},
		{"unknown", "equals"},
	}
	for _, tt := range tests {
		if got := mapNumberOp(tt.op); got != tt.want {
			t.Errorf("mapNumberOp(%q) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestMapDateOp(t *testing.T) {
	tests := []struct{ op, want string }{
		{"eq", "equals"},
		{"gt", "on_or_after"},
		{"gte", "on_or_after"},
		{"lt", "on_or_before"},
		{"lte", "on_or_before"},
		{"neq", "does_not_equal"},
		{"unknown", "equals"},
	}
	for _, tt := range tests {
		if got := mapDateOp(tt.op); got != tt.want {
			t.Errorf("mapDateOp(%q) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestExtractSchemaOptions(t *testing.T) {
	tests := []struct {
		name     string
		prop     map[string]interface{}
		propType string
		want     string
	}{
		{
			name: "select with options",
			prop: map[string]interface{}{
				"select": map[string]interface{}{
					"options": []interface{}{
						map[string]interface{}{"name": "A"},
						map[string]interface{}{"name": "B"},
						map[string]interface{}{"name": "C"},
					},
				},
			},
			propType: "select",
			want:     "A, B, C",
		},
		{
			name: "multi_select with options",
			prop: map[string]interface{}{
				"multi_select": map[string]interface{}{
					"options": []interface{}{
						map[string]interface{}{"name": "tag1"},
						map[string]interface{}{"name": "tag2"},
					},
				},
			},
			propType: "multi_select",
			want:     "tag1, tag2",
		},
		{
			name:     "number no options",
			prop:     map[string]interface{}{},
			propType: "number",
			want:     "",
		},
		{
			name: "select empty options",
			prop: map[string]interface{}{
				"select": map[string]interface{}{
					"options": []interface{}{},
				},
			},
			propType: "select",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSchemaOptions(tt.prop, tt.propType)
			if got != tt.want {
				t.Errorf("extractSchemaOptions() = %q, want %q", got, tt.want)
			}
		})
	}
}
