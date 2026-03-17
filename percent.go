package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pctState int

const (
	stText         pctState = iota
	stLT                    // saw <
	stLTPct                 // saw <%
	stSingle                // inside <%..., collecting tag
	stSingleClose           // inside single tag, saw %
	stLTPctPct              // saw <%%
	stDouble                // inside <%%..., collecting tag
	stDoubleClose1          // inside double tag, saw %
	stDoubleClose2          // inside double tag, saw %%
	stCmtBang               // saw <!
	stCmtDash1              // saw <!-
	stCmtDash2              // saw <!--
	stCmtPct1               // saw <!--%
	stCmtSingle             // inside <!--%..., collecting single tag
	stCmtSClose1            // inside single comment tag, saw %
	stCmtSClose2            // inside single comment tag, saw %-
	stCmtSClose3            // inside single comment tag, saw %--
	stCmtTag                // inside <!--%%..., collecting double tag
	stCmtClose1             // inside double comment tag, saw %
	stCmtClose2             // inside double comment tag, saw %%
	stCmtClose3             // inside double comment tag, saw %%-
	stCmtClose4             // inside double comment tag, saw %%--
	stSCmtD1                // inside single tag, saw %-
	stSCmtD2                // inside single tag, saw %--
	stDCmtD1                // inside double tag, saw %%-
	stDCmtD2                // inside double tag, saw %%--
)

type blockKind int

const (
	blockCond blockKind = iota
	blockSave
)

type blockFrame struct {
	kind     blockKind
	anyTaken bool // cond: any branch taken
	current  bool // cond: current branch emitting
	filename string           // save: target file
	saveMode string           // save: "append", "overwrite", or "" (use default)
	ltrim    bool             // save: dedent content (remove common leading whitespace)
	rtrim    bool             // save: remove trailing whitespace from lines
	prevOut  *strings.Builder // save: previous output buffer
	lineMode bool             // consume full line on close
}

func shouldEmit(stack []blockFrame) bool {
	for _, f := range stack {
		if f.kind == blockCond && !f.current {
			return false
		}
	}
	return true
}

// insideAnyExport reports whether the block stack contains an active mockup-export frame.
// When true, conditional tags must not be evaluated — content flows through as-is.
func insideAnyExport(stack []blockFrame) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == blockSave && stack[i].filename != "" {
			return true
		}
	}
	return false
}

// TransformNegative replaces mockup-export...end blocks with mockup-import tags.
// Content between export and end is removed. Nested exports each produce an import tag.
// All import tags are emitted in <!--%%...%%--> syntax regardless of the original.
func TransformNegative(content string) string {
	return transformNegative(content)
}

// transformNegative replaces mockup-export...end blocks with mockup-import tags.
// Content between export and end is removed. Nested exports each produce an import tag.
// All import tags are emitted in <!--%%...%%--> syntax regardless of the original.
func transformNegative(content string) string {
	var out strings.Builder
	var tag strings.Builder
	state := stText
	tagStart := 0 // where the '<' of the current tag started
	pos := 0      // next position in content to copy from
	depth := 0    // nesting depth of mockup-export blocks

	handleTag := func(endPos int) {
		trimmed := strings.TrimSpace(tag.String())
		if ef, ok := isMockupExport(trimmed); ok {
			if depth == 0 {
				out.WriteString(content[pos:tagStart])
			}
			depth++
			out.WriteString("<!--%%mockup-import:" + ef.filename + "%%-->")
			pos = endPos
		} else if (trimmed == "end" || trimmed == "end-mockup-export" || trimmed == "end-mockup-import") && depth > 0 {
			depth--
			out.WriteString("<!--%%end-mockup-import%%-->")
			pos = endPos
		}
	}

	for i := 0; i < len(content); i++ {
		c := content[i]
		switch state {
		case stText:
			if c == '<' {
				tagStart = i
				state = stLT
			}
		case stLT:
			switch c {
			case '%':
				state = stLTPct
			case '!':
				state = stCmtBang
			default:
				state = stText
			}
		case stLTPct:
			switch c {
			case '%':
				state = stLTPctPct
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stSingle
			}
		case stSingle:
			switch c {
			case '%':
				state = stSingleClose
			default:
				tag.WriteByte(c)
			}
		case stSingleClose:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			case '-':
				state = stSCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}
		case stLTPctPct:
			tag.Reset()
			tag.WriteByte(c)
			state = stDouble
		case stDouble:
			switch c {
			case '%':
				state = stDoubleClose1
			default:
				tag.WriteByte(c)
			}
		case stDoubleClose1:
			switch c {
			case '%':
				state = stDoubleClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stDoubleClose2:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			case '-':
				state = stDCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		case stCmtBang:
			switch c {
			case '-':
				state = stCmtDash1
			default:
				state = stText
			}
		case stCmtDash1:
			switch c {
			case '-':
				state = stCmtDash2
			default:
				state = stText
			}
		case stCmtDash2:
			switch c {
			case '%':
				state = stCmtPct1
			default:
				state = stText
			}
		case stCmtPct1:
			switch c {
			case '%':
				tag.Reset()
				state = stCmtTag
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSingle:
			switch c {
			case '%':
				state = stCmtSClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtSClose1:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			case '-':
				state = stCmtSClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose2:
			switch c {
			case '-':
				state = stCmtSClose3
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtSClose3:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stCmtSingle
			}
		case stCmtTag:
			switch c {
			case '%':
				state = stCmtClose1
			default:
				tag.WriteByte(c)
			}
		case stCmtClose1:
			switch c {
			case '%':
				state = stCmtClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose2:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			case '-':
				state = stCmtClose3
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose3:
			switch c {
			case '-':
				state = stCmtClose4
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stCmtTag
			}
		case stCmtClose4:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stCmtTag
			}

		// <%...%--> (single open, comment close)
		case stSCmtD1:
			switch c {
			case '-':
				state = stSCmtD2
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stSingle
			}
		case stSCmtD2:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stSingle
			}

		// <%%...%%--> (double open, comment close)
		case stDCmtD1:
			switch c {
			case '-':
				state = stDCmtD2
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stDouble
			}
		case stDCmtD2:
			switch c {
			case '>':
				handleTag(i + 1)
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stDouble
			}
		}
	}

	// Flush remaining content
	if depth == 0 && pos < len(content) {
		out.WriteString(content[pos:])
	}

	return out.String()
}

func (ms *Miniskin) resolvePercent(content string, vars map[string]string, chain []string) (string, error) {
	var mainOut strings.Builder
	out := &mainOut
	var tag strings.Builder
	var blockStack []blockFrame
	state := stText
	emit := true // cached shouldEmit result
	ms.consumeUntilNewline = false

	for i := 0; i < len(content); i++ {
		if ms.consumeUntilNewline {
			if content[i] == '\n' {
				ms.consumeUntilNewline = false
			}
			continue
		}
		c := content[i]

		switch state {
		case stText:
			if c == '<' {
				state = stLT
			} else if emit {
				out.WriteByte(c)
			}

		case stLT:
			switch c {
			case '%':
				state = stLTPct
			case '!':
				state = stCmtBang
			default:
				if emit {
					out.WriteByte('<')
					out.WriteByte(c)
				}
				state = stText
			}

		case stLTPct:
			switch c {
			case '%':
				state = stLTPctPct
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stSingle
			}

		case stSingle:
			switch c {
			case '%':
				state = stSingleClose
			default:
				tag.WriteByte(c)
			}

		case stSingleClose:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchSingleTag(tag.String(), vars, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			case '-':
				state = stSCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}

		case stLTPctPct:
			tag.Reset()
			tag.WriteByte(c)
			state = stDouble

		case stDouble:
			switch c {
			case '%':
				state = stDoubleClose1
			default:
				tag.WriteByte(c)
			}

		case stDoubleClose1:
			switch c {
			case '%':
				state = stDoubleClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}

		case stDoubleClose2:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchDoubleTag(tag.String(), vars, chain, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			case '-':
				state = stDCmtD1
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}

		// <%...%-->  (single open, comment close)
		case stSCmtD1:
			switch c {
			case '-':
				state = stSCmtD2
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stSingle
			}
		case stSCmtD2:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchSingleTag(tag.String(), vars, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stSingle
			}

		// <%%...%%-->  (double open, comment close)
		case stDCmtD1:
			switch c {
			case '-':
				state = stDCmtD2
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stDouble
			}
		case stDCmtD2:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchDoubleTag(tag.String(), vars, chain, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stDouble
			}

		case stCmtBang:
			switch c {
			case '-':
				state = stCmtDash1
			default:
				if emit {
					out.WriteString("<!")
				}
				state = stText
				i-- // re-process c
			}

		case stCmtDash1:
			switch c {
			case '-':
				state = stCmtDash2
			default:
				if emit {
					out.WriteString("<!-")
				}
				state = stText
				i--
			}

		case stCmtDash2:
			switch c {
			case '%':
				state = stCmtPct1
			default:
				if emit {
					out.WriteString("<!--")
				}
				state = stText
				i--
			}

		case stCmtPct1:
			switch c {
			case '%':
				tag.Reset()
				state = stCmtTag
			default:
				tag.Reset()
				tag.WriteByte(c)
				state = stCmtSingle
			}

		case stCmtSingle:
			switch c {
			case '%':
				state = stCmtSClose1
			default:
				tag.WriteByte(c)
			}

		case stCmtSClose1:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchSingleTag(tag.String(), vars, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			case '-':
				state = stCmtSClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtSingle
			}

		case stCmtSClose2:
			switch c {
			case '-':
				state = stCmtSClose3
			default:
				tag.WriteString("%-")
				tag.WriteByte(c)
				state = stCmtSingle
			}

		case stCmtSClose3:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchSingleTag(tag.String(), vars, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			default:
				tag.WriteString("%--")
				tag.WriteByte(c)
				state = stCmtSingle
			}

		case stCmtTag:
			switch c {
			case '%':
				state = stCmtClose1
			default:
				tag.WriteByte(c)
			}

		case stCmtClose1:
			switch c {
			case '%':
				state = stCmtClose2
			default:
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}

		case stCmtClose2:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchDoubleTag(tag.String(), vars, chain, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			case '-':
				state = stCmtClose3
			default:
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stCmtTag
			}

		case stCmtClose3:
			switch c {
			case '-':
				state = stCmtClose4
			default:
				tag.WriteString("%%-")
				tag.WriteByte(c)
				state = stCmtTag
			}

		case stCmtClose4:
			switch c {
			case '>':
				var err error
				out, err = ms.dispatchDoubleTag(tag.String(), vars, chain, &blockStack, out, emit)
				if err != nil {
					return "", err
				}
				emit = shouldEmit(blockStack)
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stCmtTag
			}
		}
	}

	// Check for unclosed tags
	switch state {
	case stLT:
		if emit {
			out.WriteByte('<')
		}
	case stCmtBang:
		if emit {
			out.WriteString("<!")
		}
	case stCmtDash1:
		if emit {
			out.WriteString("<!-")
		}
	case stCmtDash2:
		if emit {
			out.WriteString("<!--")
		}
	case stCmtPct1:
		if emit {
			out.WriteString("<!--%")
		}
	case stLTPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stSingle, stSingleClose, stSCmtD1, stSCmtD2:
		return "", fmt.Errorf("unclosed <%% tag: <%s", tag.String())
	case stLTPctPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stDouble, stDoubleClose1, stDoubleClose2, stDCmtD1, stDCmtD2:
		return "", fmt.Errorf("unclosed <%%%% tag: <%%%s", tag.String())
	case stCmtSingle, stCmtSClose1, stCmtSClose2, stCmtSClose3:
		return "", fmt.Errorf("unclosed <!--%%%% tag: <!--%%%s", tag.String())
	case stCmtTag, stCmtClose1, stCmtClose2, stCmtClose3, stCmtClose4:
		return "", fmt.Errorf("unclosed <!--%%%% tag: <!--%%%%%s", tag.String())
	}

	if len(blockStack) > 0 {
		return "", fmt.Errorf("unclosed block (%d levels)", len(blockStack))
	}

	return mainOut.String(), nil
}

// ---

// exportFlags holds parsed mockup-export options.
type exportFlags struct {
	filename string
	saveMode string // "append", "overwrite", or ""
	ltrim    bool
	rtrim    bool
}

// isMockupExport parses a mockup-export tag.
// Supports: mockup-export:/path, mockup-export:"/path with spaces", mockup-export:/path append ltrim
func isMockupExport(tagStr string) (ef exportFlags, ok bool) {
	if !strings.HasPrefix(tagStr, "mockup-export:") {
		return ef, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(tagStr, "mockup-export:"))
	if rest == "" {
		return ef, false
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			ef.filename = rest[1:]
			return ef, true
		}
		ef.filename = rest[1 : end+1]
		rest = strings.TrimSpace(rest[end+2:])
	} else {
		idx := strings.IndexByte(rest, ' ')
		if idx < 0 {
			ef.filename = rest
			return ef, true
		}
		ef.filename = rest[:idx]
		rest = strings.TrimSpace(rest[idx+1:])
	}
	for _, flag := range strings.Fields(rest) {
		switch flag {
		case "append", "overwrite":
			ef.saveMode = flag
		case "ltrim":
			ef.ltrim = true
		case "rtrim":
			ef.rtrim = true
		case "trim":
			ef.ltrim = true
			ef.rtrim = true
		}
	}
	return ef, true
}

func isMockupImport(tagStr string) (filename string, ok bool) {
	if !strings.HasPrefix(tagStr, "mockup-import:") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(tagStr, "mockup-import:"))
	if rest == "" {
		return "", false
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return rest[1:], true
		}
		return rest[1 : end+1], true
	}
	return rest, true
}

// applyMinify applies minification to content.
// Level "1": trim lines and remove empty lines.
func applyMinify(content string, level string) string {
	if level == "" || level == "0" {
		return content
	}
	// Level 1: alltrim lines + remove empty lines
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n") + "\n"
}

// applyLineEnding converts line endings.
// "lf" = \n, "crlf" = \r\n, "cr" = \r.
func applyLineEnding(content string, ending string) string {
	// Normalize to \n first
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	switch strings.ToLower(ending) {
	case "crlf":
		content = strings.ReplaceAll(content, "\n", "\r\n")
	case "cr":
		content = strings.ReplaceAll(content, "\n", "\r")
	}
	return content
}

// applyTrimFlags applies ltrim/rtrim to exported content.
// ltrim removes common leading whitespace (dedent).
// rtrim removes trailing whitespace from each line.
func applyTrimFlags(content string, ltrim, rtrim bool) string {
	if !ltrim && !rtrim {
		return content
	}
	lines := strings.Split(content, "\n")

	if ltrim {
		// Find minimum indentation of non-empty lines
		minIndent := -1
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if minIndent < 0 || indent < minIndent {
				minIndent = indent
			}
		}
		if minIndent > 0 {
			for i, line := range lines {
				if len(line) >= minIndent {
					lines[i] = line[minIndent:]
				}
			}
		}
	}

	if rtrim {
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
	}

	return strings.Join(lines, "\n")
}

// truncateCurrentLine removes everything after the last newline in the buffer.
func truncateCurrentLine(out *strings.Builder) {
	s := out.String()
	lastNL := strings.LastIndexByte(s, '\n')
	out.Reset()
	if lastNL >= 0 {
		out.WriteString(s[:lastNL+1])
	}
}

// importFilePath resolves a mockup-import filename.
// Leading "/" means relative to bucketSrc; otherwise relative to the current file's directory.
func importFilePath(filename, bucketSrc, fileDir string) string {
	if strings.HasPrefix(filename, "/") {
		return filepath.Join(bucketSrc, filepath.FromSlash(strings.TrimPrefix(filename, "/")))
	}
	return filepath.Join(fileDir, filepath.FromSlash(filename))
}

func (ms *Miniskin) resolveImportPath(filename string) string {
	fileDir := ms.currentFileDir
	if fileDir == "" {
		fileDir = ms.bucketSrc
	}
	return importFilePath(filename, ms.bucketSrc, fileDir)
}

func (ms *Miniskin) dispatchSingleTag(rawTag string, vars map[string]string, blockStack *[]blockFrame, out *strings.Builder, emit bool) (*strings.Builder, error) {
	tagStr := strings.TrimSpace(rawTag)
	if filename, ok := isMockupImport(tagStr); ok {
		if emit && !ms.skipVars {
			return nil, fmt.Errorf("mockup-import only works in mockup mode")
		}
		if emit {
			if ms.lineMode {
				truncateCurrentLine(out)
			}
			filePath := absPath(ms.resolveImportPath(filename))
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("mockup-import %s: %w", filePath, err)
			}
			out.Write(data)
			ms.importMarkOut = out
			ms.importMark = out.Len()
		}
		if ms.lineMode {
			ms.consumeUntilNewline = true
		}
		return out, nil
	}
	if ef, ok := isMockupExport(tagStr); ok {
		if emit && !ms.skipVars {
			return nil, fmt.Errorf("mockup-export only works in mockup mode")
		}
		// If activeExport is set, only process the matching export
		if ms.activeExport != "" && ef.filename != ms.activeExport {
			if ms.lineMode {
				truncateCurrentLine(out)
			}
			// Push a cond frame to suppress all content inside
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: false, lineMode: ms.lineMode})
			if ms.lineMode {
				ms.consumeUntilNewline = true
			}
			return out, nil
		}
		if ms.lineMode {
			truncateCurrentLine(out)
		}
		*blockStack = append(*blockStack, blockFrame{kind: blockSave, filename: ef.filename, saveMode: ef.saveMode, ltrim: ef.ltrim, rtrim: ef.rtrim, prevOut: out, lineMode: ms.lineMode})
		if ms.lineMode {
			ms.consumeUntilNewline = true
		}
		if emit {
			buf := new(strings.Builder)
			return buf, nil
		}
		return out, nil
	}
	// end-mockup-import: close a mockup-import block (truncate inline content)
	// or fall through to close a mockup-export block (blockSave).
	if tagStr == "end-mockup-import" {
		// Import mark active on same buffer → truncate inline content
		if ms.importMarkOut == out {
			content := out.String()
			out.Reset()
			out.WriteString(content[:ms.importMark])
			ms.importMarkOut = nil
			if ms.lineMode {
				ms.consumeUntilNewline = true
			}
			return out, nil
		}
		// Close a blockSave (mockup-export closed with end-mockup-import)
		if len(*blockStack) > 0 && (*blockStack)[len(*blockStack)-1].kind == blockSave {
			// Fall through to block-close logic
		} else if !emit {
			// Inside false conditional: skip silently
			return out, nil
		} else {
			return nil, fmt.Errorf("<%send-mockup-import%%> without matching mockup-import", "%")
		}
	}
	if tagStr == "end" || tagStr == "end-if" || tagStr == "end-mockup-export" || tagStr == "end-mockup-import" {
		if len(*blockStack) == 0 {
			return nil, fmt.Errorf("<%send%%> without matching block", "%")
		}
		top := (*blockStack)[len(*blockStack)-1]
		if tagStr == "end-if" && top.kind != blockCond {
			return nil, fmt.Errorf("<%send-if%%> but current block is not an if", "%")
		}
		if tagStr == "end-mockup-export" && top.kind != blockSave {
			return nil, fmt.Errorf("<%send-mockup-export%%> but current block is not a mockup-export", "%")
		}
		*blockStack = (*blockStack)[:len(*blockStack)-1]
		if top.kind == blockSave {
			if top.lineMode {
				truncateCurrentLine(out)
			}
			// Clear stale import mark if it was on this save buffer
			if ms.importMarkOut == out {
				ms.importMarkOut = nil
			}
			if top.filename == "" {
				// Skipped export (activeExport filtering) — just pop
				if top.lineMode {
					ms.consumeUntilNewline = true
				}
				return top.prevOut, nil
			}
			if emit {
				exportContent := applyTrimFlags(out.String(), top.ltrim, top.rtrim)
				filePath := absPath(filepath.Join(ms.bucketSrc, filepath.FromSlash(strings.TrimPrefix(top.filename, "/"))))
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					return nil, fmt.Errorf("mockup-export %s: creating directory: %w", filePath, err)
				}
				mode := top.saveMode
				if mode == "" {
					mode = ms.defaultSaveMode
				}
				// First write in this session always truncates
				firstTime := ms.touchedFiles != nil && !ms.touchedFiles[top.filename]
				if firstTime {
					if err := os.WriteFile(filePath, []byte(exportContent), 0644); err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", filePath, err)
					}
					ms.touchedFiles[top.filename] = true
				} else if mode == "append" {
					f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", filePath, err)
					}
					_, err = f.WriteString(exportContent)
					f.Close()
					if err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", filePath, err)
					}
				} else {
					if err := os.WriteFile(filePath, []byte(exportContent), 0644); err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", filePath, err)
					}
				}
				ms.generatedFiles = append(ms.generatedFiles, GeneratedFile{
					File:   top.filename,
					Source: ms.currentSource,
				})
				ms.logf("    mockup-export: %s (from: %s, mode: %s)", top.filename, ms.currentSource, mode)
			}
			if top.lineMode {
				ms.consumeUntilNewline = true
			}
			return top.prevOut, nil
		}
		// blockCond — just pop (same as endif)
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%" + tagStr + "%>")
		}
		if top.lineMode {
			truncateCurrentLine(out)
			ms.consumeUntilNewline = true
		}
		return out, nil
	}
	if err := ms.handleSingleTag(rawTag, vars, blockStack, out, emit); err != nil {
		return nil, err
	}
	return out, nil
}

func (ms *Miniskin) dispatchDoubleTag(rawTag string, vars map[string]string, chain []string, blockStack *[]blockFrame, out *strings.Builder, emit bool) (*strings.Builder, error) {
	trimmed := strings.TrimSpace(rawTag)
	if _, ok := isMockupImport(trimmed); ok {
		return ms.dispatchSingleTag(rawTag, vars, blockStack, out, emit)
	}
	if _, ok := isMockupExport(trimmed); ok || trimmed == "end" || trimmed == "end-if" || trimmed == "end-mockup-export" || trimmed == "end-mockup-import" {
		return ms.dispatchSingleTag(rawTag, vars, blockStack, out, emit)
	}
	if err := ms.handleDoubleTag(rawTag, vars, chain, blockStack, out, emit); err != nil {
		return nil, err
	}
	return out, nil
}

func (ms *Miniskin) handleSingleTag(rawTag string, vars map[string]string, blockStack *[]blockFrame, out *strings.Builder, emit bool) error {
	tagStr := strings.TrimSpace(rawTag)
	switch {
	case strings.HasPrefix(tagStr, "if-not:"):
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%" + tagStr + "%>")
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: true})
		} else if emit {
			varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "if-not:"))
			val, ok := vars[varName]
			active := !ok || val == ""
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: active, current: active})
		} else {
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: false})
		}

	case strings.HasPrefix(tagStr, "if:"):
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%" + tagStr + "%>")
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: true})
		} else if emit {
			varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "if:"))
			val, ok := vars[varName]
			active := ok && val != ""
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: active, current: active})
		} else {
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: false})
		}

	case strings.HasPrefix(tagStr, "elseif-not:"):
		top := topCond(*blockStack)
		if top == nil {
			return fmt.Errorf("<%selseif-not:...%%> without matching <%sif:...%%>", "%", "%")
		}
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%" + tagStr + "%>")
			top.current = true
			top.anyTaken = true
		} else if top.anyTaken {
			top.current = false
		} else {
			varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "elseif-not:"))
			val, ok := vars[varName]
			active := !ok || val == ""
			top.current = active
			top.anyTaken = active
		}

	case strings.HasPrefix(tagStr, "elseif:"):
		top := topCond(*blockStack)
		if top == nil {
			return fmt.Errorf("<%selseif:...%%> without matching <%sif:...%%>", "%", "%")
		}
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%" + tagStr + "%>")
			top.current = true
			top.anyTaken = true
		} else if top.anyTaken {
			top.current = false
		} else {
			varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "elseif:"))
			val, ok := vars[varName]
			active := ok && val != ""
			top.current = active
			top.anyTaken = active
		}

	case tagStr == "else":
		top := topCond(*blockStack)
		if top == nil {
			return fmt.Errorf("<%selse%%> without matching <%sif:...%%>", "%", "%")
		}
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%else%>")
			top.current = true
			top.anyTaken = true
		} else if top.anyTaken {
			top.current = false
		} else {
			top.current = true
			top.anyTaken = true
		}

	case tagStr == "endif":
		top := topCond(*blockStack)
		if top == nil {
			return fmt.Errorf("<%sendif%%> without matching <%sif:...%%>", "%", "%")
		}
		if emit && insideAnyExport(*blockStack) {
			out.WriteString("<%endif%>")
		}
		*blockStack = (*blockStack)[:len(*blockStack)-1]

	case strings.HasPrefix(tagStr, "note:"):
		if ms.skipVars {
			if emit {
				out.WriteString("<%" + tagStr + "%>")
			}
		}
		// normal mode: discard silently

	case strings.HasPrefix(tagStr, "echo:"):
		if emit {
			if ms.skipVars {
				out.WriteString("<%" + tagStr + "%>")
			} else {
				idx := strings.Index(rawTag, "echo:")
				if ms.defaultEscapeFn != nil {
					out.WriteString(ms.defaultEscapeFn(rawTag[idx+5:]))
				} else {
					out.WriteString(rawTag[idx+5:])
				}
			}
		}

	default:
		if emit {
			if escapeFn, rest, ok := parseEscapeTag(tagStr); ok {
				// Check for escape:echo:text pattern
				if strings.HasPrefix(rest, "echo:") {
					idx := strings.Index(rawTag, "echo:")
					out.WriteString(escapeFn(rawTag[idx+5:]))
				} else if ms.skipVars {
					out.WriteString("<%" + tagStr + "%>")
				} else {
					val, exists := vars[rest]
					if !exists {
						return fmt.Errorf("undefined variable <%%%s%%>", tagStr)
					}
					out.WriteString(escapeFn(val))
				}
			} else if ms.skipVars {
				out.WriteString("<%" + tagStr + "%>")
			} else {
				resolved, err := ms.resolveSingleTag(tagStr, vars)
				if err != nil {
					return err
				}
				out.WriteString(resolved)
			}
		}
	}
	return nil
}

func topCond(stack []blockFrame) *blockFrame {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == blockCond {
			return &stack[i]
		}
	}
	return nil
}

// ---

func (ms *Miniskin) handleDoubleTag(rawTag string, vars map[string]string, chain []string, blockStack *[]blockFrame, out *strings.Builder, emit bool) error {
	trimmed := strings.TrimSpace(rawTag)
	switch {
	case strings.HasPrefix(trimmed, "if-not:"),
		strings.HasPrefix(trimmed, "if:"),
		strings.HasPrefix(trimmed, "elseif-not:"),
		strings.HasPrefix(trimmed, "elseif:"),
		trimmed == "else",
		trimmed == "endif":
		return ms.handleSingleTag(rawTag, vars, blockStack, out, emit)
	case strings.HasPrefix(trimmed, "note:"):
		if ms.skipVars {
			if emit {
				out.WriteString("<%%" + trimmed + "%%>")
			}
		}
		// normal mode: discard silently
		return nil
	case strings.HasPrefix(trimmed, "echo:"):
		if emit {
			if ms.skipVars {
				out.WriteString("<%%" + trimmed + "%%>")
			} else {
				idx := strings.Index(rawTag, "echo:")
				out.WriteString(rawTag[idx+5:])
			}
		}
		return nil
	default:
		if emit {
			if escapeFn, rest, ok := parseEscapeTag(trimmed); ok {
				// Check for escape:echo:text pattern
				if strings.HasPrefix(rest, "echo:") {
					idx := strings.Index(rawTag, "echo:")
					out.WriteString(escapeFn(rawTag[idx+5:]))
				} else if ms.skipVars {
					out.WriteString("<%%" + trimmed + "%%>")
				} else {
					val, exists := vars[rest]
					if !exists {
						return fmt.Errorf("undefined variable <%%%s%%>", trimmed)
					}
					out.WriteString(escapeFn(val))
				}
			} else if ms.skipVars {
				out.WriteString("<%%" + trimmed + "%%>")
			} else {
				resolved, err := ms.resolveDoubleTag(trimmed, vars, chain)
				if err != nil {
					return err
				}
				out.WriteString(resolved)
			}
		}
		return nil
	}
}

// ---

func (ms *Miniskin) resolveSingleTag(name string, vars map[string]string) (string, error) {
	val, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("undefined variable <%%%s%%>", name)
	}
	if ms.defaultEscapeFn != nil {
		return ms.defaultEscapeFn(val), nil
	}
	return val, nil
}

// ---

func (ms *Miniskin) resolveDoubleTag(name string, vars map[string]string, chain []string) (string, error) {
	name = strings.TrimSpace(name)

	if strings.HasPrefix(name, "include:") {
		includePath := strings.TrimSpace(strings.TrimPrefix(name, "include:"))
		return ms.resolveInclude(includePath, vars, chain)
	}

	val, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("undefined variable <%%%s%%>", name)
	}
	return val, nil
}

// ---

func (ms *Miniskin) resolveInclude(includePath string, vars map[string]string, chain []string) (string, error) {
	var fullPath string
	if strings.HasPrefix(includePath, "/") {
		// Absolute: relative to bucket src (fallback: content path)
		base := ms.bucketSrc
		if base == "" {
			base = ms.contentPath
		}
		fullPath = filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(includePath, "/")))
	} else if len(chain) > 0 {
		// Relative: relative to the directory of the current source file
		currentDir := filepath.Dir(chain[len(chain)-1])
		fullPath = filepath.Join(currentDir, filepath.FromSlash(includePath))
	} else {
		// Fallback: relative to contentPath
		fullPath = filepath.Join(ms.contentPath, filepath.FromSlash(includePath))
	}
	fullPath, _ = filepath.Abs(fullPath)

	for _, c := range chain {
		if c == fullPath {
			return "", fmt.Errorf("inclusion cycle detected: %s", fullPath)
		}
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("include %s (resolved: %s): %w", includePath, fullPath, err)
	}

	newChain := append(chain, fullPath)
	resolved, err := ms.resolvePercent(string(data), vars, newChain)
	if err != nil {
		return "", fmt.Errorf("in include %s: %w", includePath, err)
	}

	return resolved, nil
}
