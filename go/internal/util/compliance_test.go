package util

import (
	"strings"
	"testing"
)

func TestCheckCompliance_Schema(t *testing.T) {
	specContent := `
# Database Schema

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE INDEX idx_users_email ON users (email);
CREATE UNIQUE INDEX idx_users_created ON users USING btree (created_at);
`

	opts := ComplianceCheckOptions{
		SpecContent: specContent,
		SpecPath:    "spec.md",
		CheckSchema: true,
	}

	report := CheckCompliance(opts)

	if report == nil {
		t.Fatal("Expected non-nil report")
	}

	if report.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", report.Version)
	}

	if report.Summary == nil {
		t.Fatal("Expected non-nil summary")
	}

	if report.Summary.TotalGaps == 0 {
		t.Error("Expected gaps for schema elements without migrations dir")
	}

	foundTable := false
	for _, gap := range report.Gaps {
		if gap.Category == CategorySchema && strings.Contains(gap.SpecRequirement, "users") {
			foundTable = true
			break
		}
	}
	if !foundTable {
		t.Error("Expected gap for users table")
	}
}

func TestCheckCompliance_AllCategories(t *testing.T) {
	specContent := `
# API Spec

## Endpoints
GET /api/users
POST /api/users
DELETE /api/users/{id}

## Environment Variables
| VAR_NAME | type | description |
|----------|------|-------------|
| DATABASE_URL | string | Database connection |
| API_KEY | string | API key |

## Observability
| metric_name | counter | description |
| request_total | counter | Total requests |
| response_time | histogram | Response times |

span: user_auth
span: db_query
`

	opts := ComplianceCheckOptions{
		SpecContent:        specContent,
		SpecPath:           "spec.md",
		CheckSchema:        false,
		CheckConfig:        true,
		CheckAPI:           true,
		CheckObservability: true,
	}

	report := CheckCompliance(opts)

	if report == nil {
		t.Fatal("Expected non-nil report")
	}

	hasConfigGap := false
	hasAPIGap := false
	hasObsGap := false

	for _, gap := range report.Gaps {
		switch gap.Category {
		case CategoryConfig:
			hasConfigGap = true
		case CategoryAPI:
			hasAPIGap = true
		case CategoryObservability:
			hasObsGap = true
		}
	}

	if !hasConfigGap {
		t.Error("Expected config compliance gaps")
	}
	if !hasAPIGap {
		t.Error("Expected API compliance gaps")
	}
	if !hasObsGap {
		t.Error("Expected observability compliance gaps")
	}
}

func TestExtractDDLElements(t *testing.T) {
	specContent := `
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    total DECIMAL(10,2),
    CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT chk_orders_total CHECK (total >= 0)
);

CREATE INDEX idx_orders_user ON orders (user_id);
CREATE UNIQUE INDEX idx_orders_total ON orders USING btree (total);
`

	tables, constraints, indexes := ExtractDDLElements(specContent)

	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}

	if len(tables) > 0 {
		if tables[0].Name != "orders" {
			t.Errorf("Expected table name 'orders', got '%s'", tables[0].Name)
		}
		if len(tables[0].Columns) < 3 {
			t.Errorf("Expected at least 3 columns, got %d", len(tables[0].Columns))
		}
	}

	if len(constraints) != 2 {
		t.Errorf("Expected 2 constraints, got %d", len(constraints))
	}

	foundFK := false
	foundCheck := false
	for _, c := range constraints {
		if c.ConstraintType == "FK" {
			foundFK = true
		}
		if c.ConstraintType == "CHECK" {
			foundCheck = true
		}
	}
	if !foundFK {
		t.Error("Expected FK constraint")
	}
	if !foundCheck {
		t.Error("Expected CHECK constraint")
	}

	if len(indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(indexes))
	}
}

func TestExtractConfigRequirements(t *testing.T) {
	specContent := `
## Configuration

| name | type | default |
|------|------|---------|
| DATABASE_URL | string | required |
| PORT | int | 8080 |
| DEBUG_MODE | bool | false |
`

	configs := ExtractConfigRequirements(specContent)

	if len(configs) != 3 {
		t.Errorf("Expected 3 config vars, got %d", len(configs))
	}

	foundDB := false
	foundPort := false
	for _, c := range configs {
		if c.Name == "DATABASE_URL" {
			foundDB = true
			if c.VarType != "string" {
				t.Errorf("Expected DATABASE_URL type 'string', got '%s'", c.VarType)
			}
		}
		if c.Name == "PORT" {
			foundPort = true
			if c.VarType != "int" {
				t.Errorf("Expected PORT type 'int', got '%s'", c.VarType)
			}
		}
	}

	if !foundDB {
		t.Error("Expected DATABASE_URL config var")
	}
	if !foundPort {
		t.Error("Expected PORT config var")
	}
}

func TestExtractAPIRequirements(t *testing.T) {
	specContent := `
## API Endpoints

GET /api/v1/users - List all users
POST /api/v1/users - Create user
PUT /api/v1/users/{id} - Update user
DELETE /api/v1/users/{id} - Delete user
PATCH /api/v1/users/{id}/status - Update status
`

	endpoints := ExtractAPIRequirements(specContent)

	if len(endpoints) != 5 {
		t.Errorf("Expected 5 endpoints, got %d", len(endpoints))
	}

	methods := map[string]int{"GET": 0, "POST": 0, "PUT": 0, "DELETE": 0, "PATCH": 0}
	for _, ep := range endpoints {
		methods[ep.Method]++
		if !strings.HasPrefix(ep.Path, "/api/v1/users") {
			t.Errorf("Unexpected path: %s", ep.Path)
		}
	}

	for method, count := range methods {
		if count != 1 {
			t.Errorf("Expected 1 %s endpoint, got %d", method, count)
		}
	}
}

func TestExtractObservabilityRequirements(t *testing.T) {
	specContent := `
## Metrics

| metric_name | type |
|-------------|------|
| http_requests_total | counter |
| active_connections | gauge |
| request_duration | histogram |

counter: error_count
gauge: memory_usage

## Spans
span: handle_request
span: "database_query"
span: 'cache_lookup'
`

	metrics, spans := ExtractObservabilityRequirements(specContent)

	if len(metrics) < 3 {
		t.Errorf("Expected at least 3 metrics, got %d", len(metrics))
	}

	metricTypes := make(map[string]bool)
	for _, m := range metrics {
		metricTypes[m.MetricType] = true
	}

	if !metricTypes["counter"] {
		t.Error("Expected counter metric")
	}
	if !metricTypes["gauge"] {
		t.Error("Expected gauge metric")
	}
	if !metricTypes["histogram"] {
		t.Error("Expected histogram metric")
	}

	if len(spans) < 3 {
		t.Errorf("Expected at least 3 spans, got %d", len(spans))
	}

	spanNames := make(map[string]bool)
	for _, s := range spans {
		spanNames[s.Name] = true
	}

	if !spanNames["handle_request"] {
		t.Error("Expected handle_request span")
	}
	if !spanNames["database_query"] {
		t.Error("Expected database_query span")
	}
}

func TestPathsMatch(t *testing.T) {
	tests := []struct {
		specPath string
		implPath string
		expected bool
	}{
		{"/api/users", "/api/users", true},
		{"/api/users/{id}", "/api/users/{user_id}", true},
		{"/api/users/{id}/orders/{oid}", "/api/users/{user_id}/orders/{order_id}", true},
		{"/api/users", "/api/items", false},
		{"/api/users/{id}", "/api/users", false},
		{"/api/v1/users", "/api/v2/users", false},
	}

	for _, tt := range tests {
		result := pathsMatch(tt.specPath, tt.implPath)
		if result != tt.expected {
			t.Errorf("pathsMatch(%q, %q) = %v, expected %v",
				tt.specPath, tt.implPath, result, tt.expected)
		}
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	slice := []string{"Users", "ORDERS", "products"}

	tests := []struct {
		item     string
		expected bool
	}{
		{"users", true},
		{"USERS", true},
		{"Users", true},
		{"orders", true},
		{"PRODUCTS", true},
		{"customers", false},
		{"", false},
	}

	for _, tt := range tests {
		result := containsIgnoreCase(slice, tt.item)
		if result != tt.expected {
			t.Errorf("containsIgnoreCase(%v, %q) = %v, expected %v",
				slice, tt.item, result, tt.expected)
		}
	}
}

func TestFormatComplianceReport(t *testing.T) {
	report := &ComplianceReport{
		Version:    "1.0",
		SpecPath:   "/path/to/spec.md",
		TargetPath: "/path/to/target",
		CheckedAt:  "2024-01-15T10:30:00Z",
		Gaps: []ComplianceGap{
			{
				ID:                   "V1-001",
				Category:             CategorySchema,
				Severity:             SeverityCritical,
				SpecRequirement:      "Table: users",
				SpecLocation:         "line 10",
				ImplementationStatus: StatusMissing,
				Details:              "Table not found",
				SuggestedFix:         "Create migration",
			},
			{
				ID:                   "V3-001",
				Category:             CategoryAPI,
				Severity:             SeverityWarning,
				SpecRequirement:      "GET /api/health",
				SpecLocation:         "line 25",
				ImplementationStatus: StatusMissing,
				Details:              "Endpoint not found",
				SuggestedFix:         "Add route handler",
			},
		},
		Summary: &ComplianceSummary{
			TotalGaps: 2,
			BySeverity: map[string]int{
				SeverityCritical: 1,
				SeverityWarning:  1,
				SeverityInfo:     0,
			},
			ByCategory: map[string]int{
				CategorySchema: 1,
				CategoryAPI:    1,
			},
			Compliant: false,
		},
	}

	output := FormatComplianceReport(report)

	expectedContains := []string{
		"COMPLIANCE CHECK REPORT",
		"/path/to/spec.md",
		"/path/to/target",
		"Total Gaps: 2",
		"Critical: 1",
		"Warning:  1",
		"NON-COMPLIANT",
		"V1: Schema Compliance",
		"V1-001",
		"Table: users",
		"V3: API Compliance",
		"V3-001",
		"GET /api/health",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s'", expected)
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	gaps := []ComplianceGap{
		{Category: CategorySchema, Severity: SeverityCritical},
		{Category: CategorySchema, Severity: SeverityWarning},
		{Category: CategoryConfig, Severity: SeverityInfo},
		{Category: CategoryAPI, Severity: SeverityCritical},
	}

	summary := generateSummary(gaps)

	if summary.TotalGaps != 4 {
		t.Errorf("Expected 4 total gaps, got %d", summary.TotalGaps)
	}

	if summary.BySeverity[SeverityCritical] != 2 {
		t.Errorf("Expected 2 critical gaps, got %d", summary.BySeverity[SeverityCritical])
	}

	if summary.BySeverity[SeverityWarning] != 1 {
		t.Errorf("Expected 1 warning gap, got %d", summary.BySeverity[SeverityWarning])
	}

	if summary.BySeverity[SeverityInfo] != 1 {
		t.Errorf("Expected 1 info gap, got %d", summary.BySeverity[SeverityInfo])
	}

	if summary.ByCategory[CategorySchema] != 2 {
		t.Errorf("Expected 2 schema gaps, got %d", summary.ByCategory[CategorySchema])
	}

	if summary.Compliant {
		t.Error("Expected non-compliant due to critical gaps")
	}
}

func TestGenerateSummary_Compliant(t *testing.T) {
	gaps := []ComplianceGap{
		{Category: CategorySchema, Severity: SeverityWarning},
		{Category: CategoryConfig, Severity: SeverityInfo},
	}

	summary := generateSummary(gaps)

	if !summary.Compliant {
		t.Error("Expected compliant with no critical gaps")
	}
}
