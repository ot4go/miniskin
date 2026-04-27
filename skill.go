package miniskin

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

//go:embed ai/claude.skill.template.md
var skillTemplate string

//go:embed ai/core/overview.md
var skillOverview string

//go:embed ai/core/syntax.md
var skillSyntax string

//go:embed ai/core/pipeline.md
var skillPipeline string

//go:embed ai/core/rules.md
var skillRules string

//go:embed ai/core/examples.md
var skillExamples string

// GenerateSkill produces the Claude Code SKILL.md content from embedded sources.
func GenerateSkill() (string, error) {
	tmpl, err := template.New("skill").Parse(skillTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Overview string
		Syntax   string
		Pipeline string
		Rules    string
		Examples string
	}{
		Overview: strings.TrimRight(skillOverview, "\n"),
		Syntax:   strings.TrimRight(skillSyntax, "\n"),
		Pipeline: strings.TrimRight(skillPipeline, "\n"),
		Rules:    strings.TrimRight(skillRules, "\n"),
		Examples: strings.TrimRight(skillExamples, "\n"),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GenerateAgentDocs produces an agent-agnostic Markdown document concatenating
// the embedded ai/core sources without any tool-specific frontmatter. Suitable
// for AGENTS.md, .cursor/rules, CONVENTIONS.md, or any LLM context input.
func GenerateAgentDocs() string {
	parts := []string{
		"# miniskin",
		strings.TrimRight(skillOverview, "\n"),
		strings.TrimRight(skillSyntax, "\n"),
		strings.TrimRight(skillPipeline, "\n"),
		strings.TrimRight(skillExamples, "\n"),
		strings.TrimRight(skillRules, "\n"),
	}
	return strings.Join(parts, "\n\n") + "\n"
}
