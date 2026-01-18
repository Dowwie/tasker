package util

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Severity levels for compliance gaps.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Implementation status values.
const (
	StatusMissing   = "missing"
	StatusPartial   = "partial"
	StatusDifferent = "different"
)

// Compliance check categories.
const (
	CategorySchema        = "schema"
	CategoryConfig        = "config"
	CategoryAPI           = "api"
	CategoryObservability = "observability"
)

// ComplianceGap represents a detected compliance gap between spec and implementation.
type ComplianceGap struct {
	ID                   string `json:"id"`
	Category             string `json:"category"`
	Severity             string `json:"severity"`
	SpecRequirement      string `json:"spec_requirement"`
	SpecLocation         string `json:"spec_location"`
	ImplementationStatus string `json:"implementation_status"`
	Details              string `json:"details,omitempty"`
	SuggestedFix         string `json:"suggested_fix,omitempty"`
}

// ComplianceSummary summarizes the compliance check results.
type ComplianceSummary struct {
	TotalGaps  int            `json:"total_gaps"`
	BySeverity map[string]int `json:"by_severity"`
	ByCategory map[string]int `json:"by_category"`
	Compliant  bool           `json:"compliant"`
}

// ComplianceReport contains the complete compliance check result.
type ComplianceReport struct {
	Version    string             `json:"version"`
	SpecPath   string             `json:"spec_path"`
	TargetPath string             `json:"target_path,omitempty"`
	CheckedAt  string             `json:"checked_at"`
	Gaps       []ComplianceGap    `json:"gaps"`
	Summary    *ComplianceSummary `json:"summary"`
}

// ComplianceCheckOptions configures which compliance checks to run.
type ComplianceCheckOptions struct {
	SpecContent     string
	SpecPath        string
	MigrationsDir   string
	SettingsPath    string
	RoutesPath      string
	CodePath        string
	CheckSchema     bool
	CheckConfig     bool
	CheckAPI        bool
	CheckObservability bool
}

// TableDef represents a table definition extracted from spec.
type TableDef struct {
	Name     string
	Columns  []ColumnDef
	Location string
}

// ColumnDef represents a column definition.
type ColumnDef struct {
	Name    string
	Type    string
	NotNull bool
}

// ConstraintDef represents a constraint definition.
type ConstraintDef struct {
	Name           string
	Table          string
	ConstraintType string
	Columns        []string
	Expression     string
	Location       string
}

// IndexDef represents an index definition.
type IndexDef struct {
	Name      string
	Table     string
	Columns   []string
	IndexType string
	Location  string
}

// ConfigVar represents an environment variable requirement.
type ConfigVar struct {
	Name        string
	VarType     string
	Default     string
	Required    bool
	Description string
	Location    string
}

// EndpointDef represents an API endpoint definition.
type EndpointDef struct {
	Method      string
	Path        string
	Description string
	Location    string
}

// MetricDef represents a metric definition.
type MetricDef struct {
	Name        string
	MetricType  string
	Description string
	Location    string
}

// SpanDef represents a tracing span definition.
type SpanDef struct {
	Name      string
	Operation string
	Location  string
}

// CheckCompliance runs all configured compliance checks and returns a report.
func CheckCompliance(opts ComplianceCheckOptions) *ComplianceReport {
	var gaps []ComplianceGap

	if opts.CheckSchema {
		schemaGaps := checkSchemaCompliance(opts.SpecContent, opts.MigrationsDir)
		gaps = append(gaps, schemaGaps...)
	}

	if opts.CheckConfig {
		configGaps := checkConfigCompliance(opts.SpecContent, opts.SettingsPath)
		gaps = append(gaps, configGaps...)
	}

	if opts.CheckAPI {
		apiGaps := checkAPICompliance(opts.SpecContent, opts.RoutesPath)
		gaps = append(gaps, apiGaps...)
	}

	if opts.CheckObservability {
		obsGaps := checkObservabilityCompliance(opts.SpecContent, opts.CodePath)
		gaps = append(gaps, obsGaps...)
	}

	summary := generateSummary(gaps)

	return &ComplianceReport{
		Version:    "1.0",
		SpecPath:   opts.SpecPath,
		TargetPath: opts.CodePath,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		Gaps:       gaps,
		Summary:    summary,
	}
}

func generateSummary(gaps []ComplianceGap) *ComplianceSummary {
	bySeverity := map[string]int{
		SeverityCritical: 0,
		SeverityWarning:  0,
		SeverityInfo:     0,
	}
	byCategory := make(map[string]int)

	for _, gap := range gaps {
		bySeverity[gap.Severity]++
		byCategory[gap.Category]++
	}

	return &ComplianceSummary{
		TotalGaps:  len(gaps),
		BySeverity: bySeverity,
		ByCategory: byCategory,
		Compliant:  bySeverity[SeverityCritical] == 0,
	}
}

// ExtractDDLElements extracts DDL elements (tables, constraints, indexes) from spec content.
func ExtractDDLElements(specContent string) ([]TableDef, []ConstraintDef, []IndexDef) {
	var tables []TableDef
	var constraints []ConstraintDef
	var indexes []IndexDef

	tablePattern := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(([^;]+)\)`)
	matches := tablePattern.FindAllStringSubmatchIndex(specContent, -1)

	for _, match := range matches {
		tableName := specContent[match[2]:match[3]]
		body := specContent[match[4]:match[5]]
		lineNum := countLines(specContent[:match[0]]) + 1

		columns := extractColumns(body)
		tables = append(tables, TableDef{
			Name:     tableName,
			Columns:  columns,
			Location: fmt.Sprintf("line %d", lineNum),
		})

		tableConstraints := extractConstraints(body, tableName, lineNum)
		constraints = append(constraints, tableConstraints...)
	}

	indexPattern := regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)(?:\s+USING\s+(\w+))?\s*\(([^)]+)\)`)
	indexMatches := indexPattern.FindAllStringSubmatch(specContent, -1)
	indexLocs := indexPattern.FindAllStringIndex(specContent, -1)

	for i, match := range indexMatches {
		lineNum := countLines(specContent[:indexLocs[i][0]]) + 1
		indexType := "btree"
		if match[4] != "" {
			indexType = match[4]
		}
		indexes = append(indexes, IndexDef{
			Name:      match[2],
			Table:     match[3],
			Columns:   parseColumns(match[5]),
			IndexType: indexType,
			Location:  fmt.Sprintf("line %d", lineNum),
		})
	}

	return tables, constraints, indexes
}

func extractColumns(body string) []ColumnDef {
	var columns []ColumnDef
	colPattern := regexp.MustCompile(`(?im)^\s*(\w+)\s+(\w+(?:\([^)]+\))?)\s*(NOT\s+NULL)?`)
	matches := colPattern.FindAllStringSubmatch(body, -1)

	skipWords := map[string]bool{
		"CONSTRAINT": true, "PRIMARY": true, "FOREIGN": true,
		"UNIQUE": true, "CHECK": true,
	}

	for _, match := range matches {
		colName := match[1]
		if skipWords[strings.ToUpper(colName)] {
			continue
		}
		columns = append(columns, ColumnDef{
			Name:    colName,
			Type:    match[2],
			NotNull: match[3] != "",
		})
	}

	return columns
}

func extractConstraints(body, tableName string, lineNum int) []ConstraintDef {
	var constraints []ConstraintDef

	patterns := []struct {
		pattern        *regexp.Regexp
		constraintType string
	}{
		{regexp.MustCompile(`(?i)CONSTRAINT\s+(\w+)\s+UNIQUE\s*\(([^)]+)\)`), "UNIQUE"},
		{regexp.MustCompile(`(?i)CONSTRAINT\s+(\w+)\s+CHECK\s*\(([^)]+)\)`), "CHECK"},
		{regexp.MustCompile(`(?i)CONSTRAINT\s+(\w+)\s+PRIMARY\s+KEY\s*\(([^)]+)\)`), "PK"},
		{regexp.MustCompile(`(?i)CONSTRAINT\s+(\w+)\s+FOREIGN\s+KEY\s*\(([^)]+)\)`), "FK"},
	}

	for _, p := range patterns {
		matches := p.pattern.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			constraints = append(constraints, ConstraintDef{
				Name:           match[1],
				Table:          tableName,
				ConstraintType: p.constraintType,
				Columns:        parseColumns(match[2]),
				Expression:     match[0],
				Location:       fmt.Sprintf("line %d", lineNum),
			})
		}
	}

	return constraints
}

// ExtractConfigRequirements extracts environment variable requirements from spec content.
func ExtractConfigRequirements(specContent string) []ConfigVar {
	var configs []ConfigVar

	tablePattern := regexp.MustCompile(`\|\s*` + "`?" + `([A-Z][A-Z0-9_]+)` + "`?" + `\s*\|\s*(\w+)\s*\|([^|]*)\|`)
	matches := tablePattern.FindAllStringSubmatch(specContent, -1)
	locs := tablePattern.FindAllStringIndex(specContent, -1)

	for i, match := range matches {
		lineNum := countLines(specContent[:locs[i][0]]) + 1
		varName := match[1]
		varType := strings.ToLower(match[2])
		rest := strings.TrimSpace(match[3])

		var defaultVal string
		required := true
		if rest != "" && len(rest) > 0 && (rest[0] >= 'a' && rest[0] <= 'z' || rest[0] >= '0' && rest[0] <= '9') {
			defaultVal = rest
			lowerDefault := strings.ToLower(defaultVal)
			required = lowerDefault == "required" || lowerDefault == "none" || lowerDefault == ""
		}

		configs = append(configs, ConfigVar{
			Name:     varName,
			VarType:  varType,
			Default:  defaultVal,
			Required: required,
			Location: fmt.Sprintf("line %d", lineNum),
		})
	}

	return configs
}

// ExtractAPIRequirements extracts API endpoint requirements from spec content.
func ExtractAPIRequirements(specContent string) []EndpointDef {
	var endpoints []EndpointDef

	endpointPattern := regexp.MustCompile(`(GET|POST|PUT|PATCH|DELETE)\s+(/[^\s\n]+)`)
	matches := endpointPattern.FindAllStringSubmatch(specContent, -1)
	locs := endpointPattern.FindAllStringIndex(specContent, -1)

	for i, match := range matches {
		lineNum := countLines(specContent[:locs[i][0]]) + 1

		contextEnd := locs[i][1] + 100
		if contextEnd > len(specContent) {
			contextEnd = len(specContent)
		}
		context := specContent[locs[i][1]:contextEnd]
		if idx := strings.Index(context, "\n"); idx >= 0 {
			context = context[:idx]
		}
		context = strings.Trim(context, " -:")

		endpoints = append(endpoints, EndpointDef{
			Method:      match[1],
			Path:        match[2],
			Description: context,
			Location:    fmt.Sprintf("line %d", lineNum),
		})
	}

	return endpoints
}

// ExtractObservabilityRequirements extracts metrics and span requirements from spec content.
func ExtractObservabilityRequirements(specContent string) ([]MetricDef, []SpanDef) {
	var metrics []MetricDef
	var spans []SpanDef

	tableMetricPattern := regexp.MustCompile(`\|\s*` + "`?" + `(\w+_\w+)` + "`?" + `\s*\|\s*(counter|gauge|histogram)\s*\|`)
	matches := tableMetricPattern.FindAllStringSubmatch(specContent, -1)
	locs := tableMetricPattern.FindAllStringIndex(specContent, -1)

	for i, match := range matches {
		lineNum := countLines(specContent[:locs[i][0]]) + 1
		metrics = append(metrics, MetricDef{
			Name:       match[1],
			MetricType: strings.ToLower(match[2]),
			Location:   fmt.Sprintf("line %d", lineNum),
		})
	}

	inlineMetricPattern := regexp.MustCompile(`(?i)(counter|gauge|histogram)\s*:\s*` + "`?" + `(\w+)` + "`?")
	inlineMatches := inlineMetricPattern.FindAllStringSubmatch(specContent, -1)
	inlineLocs := inlineMetricPattern.FindAllStringIndex(specContent, -1)

	for i, match := range inlineMatches {
		lineNum := countLines(specContent[:inlineLocs[i][0]]) + 1
		metrics = append(metrics, MetricDef{
			Name:       match[2],
			MetricType: strings.ToLower(match[1]),
			Location:   fmt.Sprintf("line %d", lineNum),
		})
	}

	spanPattern := regexp.MustCompile(`(?i)span[:\s]+[` + "`'\"" + `]?(\w+)[` + "`'\"" + `]?`)
	spanMatches := spanPattern.FindAllStringSubmatch(specContent, -1)
	spanLocs := spanPattern.FindAllStringIndex(specContent, -1)

	for i, match := range spanMatches {
		lineNum := countLines(specContent[:spanLocs[i][0]]) + 1
		spans = append(spans, SpanDef{
			Name:     match[1],
			Location: fmt.Sprintf("line %d", lineNum),
		})
	}

	return metrics, spans
}

func checkSchemaCompliance(specContent, migrationsDir string) []ComplianceGap {
	var gaps []ComplianceGap
	gapCounter := 0

	tables, constraints, indexes := ExtractDDLElements(specContent)

	if migrationsDir == "" {
		for _, table := range tables {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V1-%03d", gapCounter),
				Category:             CategorySchema,
				Severity:             SeverityInfo,
				SpecRequirement:      fmt.Sprintf("Table: %s", table.Name),
				SpecLocation:         table.Location,
				ImplementationStatus: StatusMissing,
				Details:              "No migrations directory provided - cannot verify",
				SuggestedFix:         "Run with migrations directory to verify",
			})
		}
		return gaps
	}

	implTables, implConstraints, implIndexes := scanMigrationsForSchema(migrationsDir)

	for _, table := range tables {
		if !containsIgnoreCase(implTables, table.Name) {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V1-%03d", gapCounter),
				Category:             CategorySchema,
				Severity:             SeverityCritical,
				SpecRequirement:      fmt.Sprintf("Table: %s", table.Name),
				SpecLocation:         table.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Table '%s' not found in migrations", table.Name),
				SuggestedFix:         fmt.Sprintf("Add migration to create table '%s'", table.Name),
			})
		}
	}

	for _, constraint := range constraints {
		if !containsIgnoreCase(implConstraints, constraint.Name) {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V1-%03d", gapCounter),
				Category:             CategorySchema,
				Severity:             SeverityCritical,
				SpecRequirement:      fmt.Sprintf("Constraint: %s (%s)", constraint.Name, constraint.ConstraintType),
				SpecLocation:         constraint.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Constraint '%s' not found", constraint.Name),
				SuggestedFix:         fmt.Sprintf("Add %s constraint on %s(%s)", constraint.ConstraintType, constraint.Table, strings.Join(constraint.Columns, ", ")),
			})
		}
	}

	for _, index := range indexes {
		if !containsIgnoreCase(implIndexes, index.Name) {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V1-%03d", gapCounter),
				Category:             CategorySchema,
				Severity:             SeverityWarning,
				SpecRequirement:      fmt.Sprintf("Index: %s", index.Name),
				SpecLocation:         index.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Index '%s' not found", index.Name),
				SuggestedFix:         fmt.Sprintf("Add %s index on %s(%s)", index.IndexType, index.Table, strings.Join(index.Columns, ", ")),
			})
		}
	}

	return gaps
}

func checkConfigCompliance(specContent, settingsPath string) []ComplianceGap {
	var gaps []ComplianceGap
	gapCounter := 0

	configVars := ExtractConfigRequirements(specContent)
	implConfig := scanSettingsForConfig(settingsPath)

	for _, configVar := range configVars {
		if _, exists := implConfig[configVar.Name]; !exists {
			severity := SeverityWarning
			if configVar.Required {
				severity = SeverityCritical
			}
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V2-%03d", gapCounter),
				Category:             CategoryConfig,
				Severity:             severity,
				SpecRequirement:      fmt.Sprintf("Env var: %s (%s)", configVar.Name, configVar.VarType),
				SpecLocation:         configVar.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Environment variable '%s' not found in settings", configVar.Name),
				SuggestedFix:         fmt.Sprintf("Add %s: %s field to settings class", strings.ToLower(configVar.Name), configVar.VarType),
			})
		}
	}

	return gaps
}

func checkAPICompliance(specContent, routesPath string) []ComplianceGap {
	var gaps []ComplianceGap
	gapCounter := 0

	endpoints := ExtractAPIRequirements(specContent)
	implEndpoints := scanRoutesForEndpoints(routesPath)

	implSet := make(map[string]bool)
	for _, ep := range implEndpoints {
		key := fmt.Sprintf("%s %s", ep.Method, ep.Path)
		implSet[key] = true
	}

	for _, endpoint := range endpoints {
		found := false
		specPathNorm := strings.TrimRight(endpoint.Path, "/")

		for _, impl := range implEndpoints {
			if impl.Method != endpoint.Method {
				continue
			}
			implPathNorm := strings.TrimRight(impl.Path, "/")
			if pathsMatch(specPathNorm, implPathNorm) {
				found = true
				break
			}
		}

		if !found {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V3-%03d", gapCounter),
				Category:             CategoryAPI,
				Severity:             SeverityCritical,
				SpecRequirement:      fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path),
				SpecLocation:         endpoint.Location,
				ImplementationStatus: StatusMissing,
				Details:              "Endpoint not found in routes",
				SuggestedFix:         fmt.Sprintf("Add @router.%s('%s') handler", strings.ToLower(endpoint.Method), endpoint.Path),
			})
		}
	}

	return gaps
}

func checkObservabilityCompliance(specContent, codePath string) []ComplianceGap {
	var gaps []ComplianceGap
	gapCounter := 0

	metrics, spans := ExtractObservabilityRequirements(specContent)
	implMetrics, implSpans := scanCodeForObservability(codePath)

	for _, metric := range metrics {
		if !containsIgnoreCase(implMetrics, metric.Name) {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V4-%03d", gapCounter),
				Category:             CategoryObservability,
				Severity:             SeverityWarning,
				SpecRequirement:      fmt.Sprintf("Metric: %s (%s)", metric.Name, metric.MetricType),
				SpecLocation:         metric.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Metric '%s' not found in code", metric.Name),
				SuggestedFix:         fmt.Sprintf("Add meter.create_%s('%s')", metric.MetricType, metric.Name),
			})
		}
	}

	for _, span := range spans {
		if !containsIgnoreCase(implSpans, span.Name) {
			gapCounter++
			gaps = append(gaps, ComplianceGap{
				ID:                   fmt.Sprintf("V4-%03d", gapCounter),
				Category:             CategoryObservability,
				Severity:             SeverityInfo,
				SpecRequirement:      fmt.Sprintf("Span: %s", span.Name),
				SpecLocation:         span.Location,
				ImplementationStatus: StatusMissing,
				Details:              fmt.Sprintf("Span '%s' not found in code", span.Name),
				SuggestedFix:         fmt.Sprintf("Add tracer.start_span('%s')", span.Name),
			})
		}
	}

	return gaps
}

func scanMigrationsForSchema(migrationsDir string) ([]string, []string, []string) {
	return []string{}, []string{}, []string{}
}

func scanSettingsForConfig(settingsPath string) map[string]map[string]interface{} {
	return make(map[string]map[string]interface{})
}

func scanRoutesForEndpoints(routesPath string) []EndpointDef {
	return []EndpointDef{}
}

func scanCodeForObservability(codePath string) ([]string, []string) {
	return []string{}, []string{}
}

func countLines(s string) int {
	return strings.Count(s, "\n")
}

func parseColumns(colStr string) []string {
	parts := strings.Split(colStr, ",")
	var columns []string
	for _, p := range parts {
		col := strings.TrimSpace(p)
		if spaceIdx := strings.Index(col, " "); spaceIdx > 0 {
			col = col[:spaceIdx]
		}
		if col != "" {
			columns = append(columns, col)
		}
	}
	return columns
}

func containsIgnoreCase(slice []string, item string) bool {
	itemLower := strings.ToLower(item)
	for _, s := range slice {
		if strings.ToLower(s) == itemLower {
			return true
		}
	}
	return false
}

func pathsMatch(specPath, implPath string) bool {
	specParts := strings.Split(specPath, "/")
	implParts := strings.Split(implPath, "/")

	if len(specParts) != len(implParts) {
		return false
	}

	for i := range specParts {
		sp := specParts[i]
		ip := implParts[i]

		if strings.HasPrefix(sp, "{") && strings.HasPrefix(ip, "{") {
			continue
		}

		if sp != ip {
			return false
		}
	}

	return true
}

// FormatComplianceReport formats a ComplianceReport as a human-readable string.
func FormatComplianceReport(report *ComplianceReport) string {
	var output strings.Builder

	output.WriteString("============================================================\n")
	output.WriteString("COMPLIANCE CHECK REPORT\n")
	output.WriteString("============================================================\n\n")

	output.WriteString(fmt.Sprintf("Spec: %s\n", report.SpecPath))
	output.WriteString(fmt.Sprintf("Target: %s\n", report.TargetPath))
	output.WriteString(fmt.Sprintf("Checked: %s\n\n", report.CheckedAt))

	output.WriteString(fmt.Sprintf("Total Gaps: %d\n", report.Summary.TotalGaps))
	output.WriteString(fmt.Sprintf("  Critical: %d\n", report.Summary.BySeverity[SeverityCritical]))
	output.WriteString(fmt.Sprintf("  Warning:  %d\n", report.Summary.BySeverity[SeverityWarning]))
	output.WriteString(fmt.Sprintf("  Info:     %d\n\n", report.Summary.BySeverity[SeverityInfo]))

	if report.Summary.Compliant {
		output.WriteString("STATUS: COMPLIANT - No critical gaps\n\n")
	} else {
		output.WriteString("STATUS: NON-COMPLIANT - Critical gaps require attention\n\n")
	}

	byCat := make(map[string][]ComplianceGap)
	for _, gap := range report.Gaps {
		byCat[gap.Category] = append(byCat[gap.Category], gap)
	}

	categoryNames := map[string]string{
		CategorySchema:        "V1: Schema Compliance",
		CategoryConfig:        "V2: Configuration Compliance",
		CategoryAPI:           "V3: API Compliance",
		CategoryObservability: "V4: Observability Compliance",
	}

	for _, cat := range []string{CategorySchema, CategoryConfig, CategoryAPI, CategoryObservability} {
		gaps, exists := byCat[cat]
		if !exists || len(gaps) == 0 {
			continue
		}

		output.WriteString("------------------------------------------------------------\n")
		output.WriteString(fmt.Sprintf("%s\n", categoryNames[cat]))
		output.WriteString("------------------------------------------------------------\n")

		for _, gap := range gaps {
			severityIcon := "."
			switch gap.Severity {
			case SeverityCritical:
				severityIcon = "!"
			case SeverityWarning:
				severityIcon = "?"
			}

			output.WriteString(fmt.Sprintf("\n[%s] %s (%s)\n", severityIcon, gap.ID, gap.SpecLocation))
			output.WriteString(fmt.Sprintf("    Requirement: %s\n", gap.SpecRequirement))
			output.WriteString(fmt.Sprintf("    Status: %s\n", gap.ImplementationStatus))
			if gap.Details != "" {
				output.WriteString(fmt.Sprintf("    Details: %s\n", gap.Details))
			}
			if gap.SuggestedFix != "" {
				output.WriteString(fmt.Sprintf("    Fix: %s\n", gap.SuggestedFix))
			}
		}
	}

	output.WriteString("\n")
	return output.String()
}
