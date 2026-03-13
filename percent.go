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
	prevOut  *strings.Builder // save: previous output buffer
}

func shouldEmit(stack []blockFrame) bool {
	for _, f := range stack {
		if f.kind == blockCond && !f.current {
			return false
		}
	}
	return true
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
		if filename, _, ok := isMockupExport(trimmed); ok {
			if depth == 0 {
				out.WriteString(content[pos:tagStart])
			}
			depth++
			out.WriteString("<!--%%mockup-import:" + filename + "%%-->")
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

	for i := 0; i < len(content); i++ {
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
	case stSingle, stSingleClose:
		return "", fmt.Errorf("unclosed <%% tag: <%s", tag.String())
	case stLTPctPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stDouble, stDoubleClose1, stDoubleClose2:
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

// isMockupExport parses a mockup-export tag.
// Supports: mockup-export:/path, mockup-export:"/path with spaces", mockup-export:/path append
func isMockupExport(tagStr string) (filename string, mode string, ok bool) {
	if !strings.HasPrefix(tagStr, "mockup-export:") {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(tagStr, "mockup-export:"))
	if rest == "" {
		return "", "", false
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return rest[1:], "", true // unclosed quote: use the rest as filename
		}
		filename = rest[1 : end+1]
		rest = strings.TrimSpace(rest[end+2:])
	} else {
		idx := strings.IndexByte(rest, ' ')
		if idx < 0 {
			return rest, "", true
		}
		filename = rest[:idx]
		rest = strings.TrimSpace(rest[idx+1:])
	}
	return filename, rest, true
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

func (ms *Miniskin) dispatchSingleTag(rawTag string, vars map[string]string, blockStack *[]blockFrame, out *strings.Builder, emit bool) (*strings.Builder, error) {
	tagStr := strings.TrimSpace(rawTag)
	if filename, ok := isMockupImport(tagStr); ok {
		if emit && !ms.skipVars {
			return nil, fmt.Errorf("mockup-import only works in mockup mode")
		}
		if emit {
			filePath := filepath.Join(ms.contentPath, filepath.FromSlash(filename))
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("mockup-import %s: %w", filename, err)
			}
			out.Write(data)
		}
		return out, nil
	}
	if filename, mode, ok := isMockupExport(tagStr); ok {
		if emit && !ms.skipVars {
			return nil, fmt.Errorf("mockup-export only works in mockup mode")
		}
		// If activeExport is set, only process the matching export
		if ms.activeExport != "" && filename != ms.activeExport {
			// Push a cond frame to suppress all content inside
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: false})
			return out, nil
		}
		*blockStack = append(*blockStack, blockFrame{kind: blockSave, filename: filename, saveMode: mode, prevOut: out})
		if emit {
			buf := new(strings.Builder)
			return buf, nil
		}
		return out, nil
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
		if tagStr == "end-mockup-import" && top.kind != blockSave {
			return nil, fmt.Errorf("<%send-mockup-import%%> but current block is not a mockup-import", "%")
		}
		*blockStack = (*blockStack)[:len(*blockStack)-1]
		if top.kind == blockSave {
			if top.filename == "" {
				// Skipped export (activeExport filtering) — just pop
				return top.prevOut, nil
			}
			if emit {
				filePath := filepath.Join(ms.contentPath, filepath.FromSlash(top.filename))
				mode := top.saveMode
				if mode == "" {
					mode = ms.defaultSaveMode
				}
				// First write in this session always truncates
				firstTime := ms.touchedFiles != nil && !ms.touchedFiles[top.filename]
				if firstTime {
					if err := os.WriteFile(filePath, []byte(out.String()), 0644); err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", top.filename, err)
					}
					ms.touchedFiles[top.filename] = true
				} else if mode == "append" {
					f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", top.filename, err)
					}
					_, err = f.WriteString(out.String())
					f.Close()
					if err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", top.filename, err)
					}
				} else {
					if err := os.WriteFile(filePath, []byte(out.String()), 0644); err != nil {
						return nil, fmt.Errorf("mockup-export %s: %w", top.filename, err)
					}
				}
				ms.generatedFiles = append(ms.generatedFiles, GeneratedFile{
					File:   top.filename,
					Source: ms.currentSource,
				})
				ms.logf("    mockup-export: %s (from: %s, mode: %s)", top.filename, ms.currentSource, mode)
			}
			return top.prevOut, nil
		}
		// blockCond — just pop (same as endif)
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
	if _, _, ok := isMockupExport(trimmed); ok || trimmed == "end" || trimmed == "end-if" || trimmed == "end-mockup-export" || trimmed == "end-mockup-import" {
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
		varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "if-not:"))
		if emit {
			val, ok := vars[varName]
			active := !ok || val == ""
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: active, current: active})
		} else {
			*blockStack = append(*blockStack, blockFrame{kind: blockCond, anyTaken: true, current: false})
		}

	case strings.HasPrefix(tagStr, "if:"):
		varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "if:"))
		if emit {
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
		if top.anyTaken {
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
		if top.anyTaken {
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
		if top.anyTaken {
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
	fullPath := filepath.Join(ms.contentPath, filepath.FromSlash(includePath))

	for _, c := range chain {
		if c == fullPath {
			return "", fmt.Errorf("inclusion cycle detected: %s", fullPath)
		}
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("include %s: %w", includePath, err)
	}

	newChain := append(chain, fullPath)
	resolved, err := ms.resolvePercent(string(data), vars, newChain)
	if err != nil {
		return "", fmt.Errorf("in include %s: %w", includePath, err)
	}

	return resolved, nil
}
