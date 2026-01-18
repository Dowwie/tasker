package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dgordon/tasker/internal/errors"
)

type GeneratedSpec struct {
	Title       string
	Slug        string
	Content     string
	OutputPath  string
	GeneratedAt string
}

type GenerateOptions struct {
	Force     bool
	TargetDir string
}

func Slugify(text string) string {
	slug := strings.ToLower(text)
	nonAlphaNum := regexp.MustCompile(`[^\w\s-]`)
	slug = nonAlphaNum.ReplaceAllString(slug, "")
	whitespace := regexp.MustCompile(`[\s_]+`)
	slug = whitespace.ReplaceAllString(slug, "-")
	multiDash := regexp.MustCompile(`-+`)
	slug = multiDash.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func GenerateSpec(session *Session, opts GenerateOptions) (*GeneratedSpec, error) {
	if session == nil {
		return nil, errors.ValidationFailed("session is required")
	}

	if session.Topic == "" {
		return nil, errors.ValidationFailed("session topic is required")
	}

	slug := Slugify(session.Topic)
	content := generateSpecContent(session, slug)

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = session.TargetDir
	}
	if targetDir == "" {
		cwd, _ := os.Getwd()
		targetDir = cwd
	}

	specsDir := filepath.Join(targetDir, "docs", "specs")
	outputPath := filepath.Join(specsDir, fmt.Sprintf("%s.md", slug))

	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return nil, errors.ValidationFailed(fmt.Sprintf("spec file already exists: %s (use --force to overwrite)", outputPath))
		}
	}

	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return nil, errors.IOWriteFailed(specsDir, err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return nil, errors.IOWriteFailed(outputPath, err)
	}

	return &GeneratedSpec{
		Title:       session.Topic,
		Slug:        slug,
		Content:     content,
		OutputPath:  outputPath,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func GenerateSpecContent(session *Session) (string, error) {
	if session == nil {
		return "", errors.ValidationFailed("session is required")
	}

	if session.Topic == "" {
		return "", errors.ValidationFailed("session topic is required")
	}

	slug := Slugify(session.Topic)
	return generateSpecContent(session, slug), nil
}

func generateSpecContent(session *Session, slug string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Spec: %s\n\n", session.Topic))

	sb.WriteString("## Related ADRs\n")
	if len(session.ADRs) > 0 {
		for _, adrID := range session.ADRs {
			sb.WriteString(fmt.Sprintf("- [ADR-%s](../adrs/ADR-%s.md)\n", adrID, adrID))
		}
	} else {
		sb.WriteString("(none)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Goal\n")
	if session.Scope.Goal != "" {
		sb.WriteString(session.Scope.Goal)
	} else {
		sb.WriteString("(goal not defined)")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Non-goals\n")
	if len(session.Scope.NonGoals) > 0 {
		for _, ng := range session.Scope.NonGoals {
			sb.WriteString(fmt.Sprintf("- %s\n", ng))
		}
	} else {
		sb.WriteString("- (none specified)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Done means\n")
	if len(session.Scope.DoneMeans) > 0 {
		for _, dm := range session.Scope.DoneMeans {
			sb.WriteString(fmt.Sprintf("- %s\n", dm))
		}
	} else {
		sb.WriteString("- (none specified)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Workflows\n")
	if session.Workflows != "" {
		sb.WriteString(session.Workflows)
	} else {
		sb.WriteString("(to be defined)")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Invariants\n")
	if len(session.Invariants) > 0 {
		for _, inv := range session.Invariants {
			sb.WriteString(fmt.Sprintf("- %s\n", inv))
		}
	} else {
		sb.WriteString("- (none specified)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Interfaces\n")
	if session.Interfaces != "" {
		sb.WriteString(session.Interfaces)
	} else {
		sb.WriteString("No new/changed interfaces")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Architecture sketch\n")
	if session.Architecture != "" {
		sb.WriteString(session.Architecture)
	} else {
		sb.WriteString("(to be defined)")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Decisions\n")
	if len(session.Decisions) > 0 {
		sb.WriteString("| Decision | ADR |\n")
		sb.WriteString("|----------|-----|\n")
		for _, d := range session.Decisions {
			adrLink := "(inline)"
			if d.ADRID != "" {
				adrLink = fmt.Sprintf("[ADR-%s](../adrs/ADR-%s.md)", d.ADRID, d.ADRID)
			}
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", d.Decision, adrLink))
		}
	} else {
		sb.WriteString("(no decisions recorded)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Open Questions\n\n")
	sb.WriteString("### Blocking\n")
	if len(session.OpenQuestions.Blocking) > 0 {
		for _, q := range session.OpenQuestions.Blocking {
			sb.WriteString(fmt.Sprintf("- %s\n", q))
		}
	} else {
		sb.WriteString("(none)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("### Non-blocking\n")
	if len(session.OpenQuestions.NonBlocking) > 0 {
		for _, q := range session.OpenQuestions.NonBlocking {
			sb.WriteString(fmt.Sprintf("- %s\n", q))
		}
	} else {
		sb.WriteString("(none)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Agent Handoff\n")
	if session.Handoff != nil && len(session.Handoff) > 0 {
		whatToBuild := session.Handoff["what_to_build"]
		if whatToBuild == "" {
			whatToBuild = "See workflows above"
		}
		mustPreserve := session.Handoff["must_preserve"]
		if mustPreserve == "" {
			mustPreserve = "See invariants above"
		}
		blockingCond := session.Handoff["blocking_conditions"]
		if blockingCond == "" {
			if len(session.OpenQuestions.Blocking) > 0 {
				blockingCond = "See blocking open questions"
			} else {
				blockingCond = "None"
			}
		}
		sb.WriteString(fmt.Sprintf("- **What to build:** %s\n", whatToBuild))
		sb.WriteString(fmt.Sprintf("- **Must preserve:** %s\n", mustPreserve))
		sb.WriteString(fmt.Sprintf("- **Blocking conditions:** %s\n", blockingCond))
	} else {
		sb.WriteString("- **What to build:** See workflows above\n")
		sb.WriteString("- **Must preserve:** See invariants above\n")
		if len(session.OpenQuestions.Blocking) > 0 {
			sb.WriteString("- **Blocking conditions:** See blocking open questions\n")
		} else {
			sb.WriteString("- **Blocking conditions:** None\n")
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Artifacts\n")
	sb.WriteString(fmt.Sprintf("- **Capability Map:** [%s.capabilities.json](./%s.capabilities.json)\n", slug, slug))
	sb.WriteString("- **Discovery Log:** [clarify-session.md](../.claude/clarify-session.md)\n")

	return sb.String()
}

type ADRInput struct {
	Number       int
	Title        string
	Context      string
	Decision     string
	Alternatives []Alternative
	Consequences []string
	AppliesTo    []SpecReference
	Supersedes   string
	RelatedADRs  []string
}

type Alternative struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type SpecReference struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type GeneratedADR struct {
	Number     int
	Title      string
	Slug       string
	Content    string
	OutputPath string
}

func GenerateADR(input ADRInput, opts GenerateOptions) (*GeneratedADR, error) {
	if input.Title == "" {
		return nil, errors.ValidationFailed("ADR title is required")
	}
	if input.Context == "" {
		return nil, errors.ValidationFailed("ADR context is required")
	}
	if input.Decision == "" {
		return nil, errors.ValidationFailed("ADR decision is required")
	}

	content := generateADRContent(input)

	targetDir := opts.TargetDir
	if targetDir == "" {
		cwd, _ := os.Getwd()
		targetDir = cwd
	}

	adrsDir := filepath.Join(targetDir, "docs", "adrs")
	adrSlug := Slugify(input.Title)
	outputPath := filepath.Join(adrsDir, fmt.Sprintf("ADR-%04d-%s.md", input.Number, adrSlug))

	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return nil, errors.ValidationFailed(fmt.Sprintf("ADR file already exists: %s (use --force to overwrite)", outputPath))
		}
	}

	if err := os.MkdirAll(adrsDir, 0755); err != nil {
		return nil, errors.IOWriteFailed(adrsDir, err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return nil, errors.IOWriteFailed(outputPath, err)
	}

	return &GeneratedADR{
		Number:     input.Number,
		Title:      input.Title,
		Slug:       adrSlug,
		Content:    content,
		OutputPath: outputPath,
	}, nil
}

func GenerateADRContent(input ADRInput) (string, error) {
	if input.Title == "" {
		return "", errors.ValidationFailed("ADR title is required")
	}
	if input.Context == "" {
		return "", errors.ValidationFailed("ADR context is required")
	}
	if input.Decision == "" {
		return "", errors.ValidationFailed("ADR decision is required")
	}

	return generateADRContent(input), nil
}

func generateADRContent(input ADRInput) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# ADR-%04d: %s\n\n", input.Number, input.Title))
	sb.WriteString("**Status:** Accepted\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", time.Now().Format("2006-01-02")))

	sb.WriteString("## Applies To\n")
	if len(input.AppliesTo) > 0 {
		for _, spec := range input.AppliesTo {
			title := spec.Title
			if title == "" {
				title = spec.Slug
			}
			sb.WriteString(fmt.Sprintf("- [%s](../specs/%s.md)\n", title, spec.Slug))
		}
	} else {
		sb.WriteString("- (none specified)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Context\n")
	sb.WriteString(input.Context)
	sb.WriteString("\n\n")

	sb.WriteString("## Decision\n")
	sb.WriteString(input.Decision)
	sb.WriteString("\n\n")

	sb.WriteString("## Alternatives Considered\n")
	if len(input.Alternatives) > 0 {
		sb.WriteString("| Alternative | Why Not Chosen |\n")
		sb.WriteString("|-------------|----------------|\n")
		for _, alt := range input.Alternatives {
			reason := alt.Reason
			if reason == "" {
				reason = "N/A"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", alt.Name, reason))
		}
	} else {
		sb.WriteString("(no alternatives recorded)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Consequences\n")
	if len(input.Consequences) > 0 {
		for _, c := range input.Consequences {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	} else {
		sb.WriteString("- (none specified)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Related\n")
	supersedes := "(none)"
	if input.Supersedes != "" {
		supersedes = fmt.Sprintf("ADR-%s", input.Supersedes)
	}
	sb.WriteString(fmt.Sprintf("- Supersedes: %s\n", supersedes))

	related := "(none)"
	if len(input.RelatedADRs) > 0 {
		refs := make([]string, len(input.RelatedADRs))
		for i, r := range input.RelatedADRs {
			refs[i] = fmt.Sprintf("ADR-%s", r)
		}
		related = strings.Join(refs, ", ")
	}
	sb.WriteString(fmt.Sprintf("- Related ADRs: %s\n", related))

	return sb.String()
}
