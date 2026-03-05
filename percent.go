package miniskin

import (
	"fmt"
	"html"
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
)

type condState struct {
	anyTaken bool // true if any branch (if/elseif) was already taken
	current  bool // true if the current branch is emitting content
}

func shouldEmit(stack []condState) bool {
	for _, s := range stack {
		if !s.current {
			return false
		}
	}
	return true
}

// resolvePercent processes <%var%> (escaped) and <%%var%%> (literal) tags
// in a single pass using a finite state machine.
func (ms *Miniskin) resolvePercent(content string, vars map[string]string, chain []string) (string, error) {
	var out strings.Builder
	var tag strings.Builder
	var condStack []condState
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
			if c == '%' {
				state = stLTPct
			} else {
				if emit {
					out.WriteByte('<')
					out.WriteByte(c)
				}
				state = stText
			}

		case stLTPct:
			if c == '%' {
				state = stLTPctPct
			} else {
				tag.Reset()
				tag.WriteByte(c)
				state = stSingle
			}

		case stSingle:
			if c == '%' {
				state = stSingleClose
			} else {
				tag.WriteByte(c)
			}

		case stSingleClose:
			if c == '>' {
				tagStr := strings.TrimSpace(tag.String())
				if err := ms.handleSingleTag(tagStr, vars, &condStack, &out, emit); err != nil {
					return "", err
				}
				emit = shouldEmit(condStack)
				state = stText
			} else {
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}

		case stLTPctPct:
			tag.Reset()
			tag.WriteByte(c)
			state = stDouble

		case stDouble:
			if c == '%' {
				state = stDoubleClose1
			} else {
				tag.WriteByte(c)
			}

		case stDoubleClose1:
			if c == '%' {
				state = stDoubleClose2
			} else {
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}

		case stDoubleClose2:
			if c == '>' {
				if emit {
					resolved, err := ms.resolveDoubleTag(tag.String(), vars, chain)
					if err != nil {
						return "", err
					}
					out.WriteString(resolved)
				}
				state = stText
			} else {
				tag.WriteByte('%')
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}
		}
	}

	// Check for unclosed tags
	switch state {
	case stLT:
		if emit {
			out.WriteByte('<')
		}
	case stLTPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stSingle, stSingleClose:
		return "", fmt.Errorf("unclosed <%% tag: <%s", tag.String())
	case stLTPctPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stDouble, stDoubleClose1, stDoubleClose2:
		return "", fmt.Errorf("unclosed <%%%% tag: <%%%s", tag.String())
	}

	if len(condStack) > 0 {
		return "", fmt.Errorf("unclosed <%sif:...%%> block (%d levels)", "%", len(condStack))
	}

	return out.String(), nil
}

// ---

func (ms *Miniskin) handleSingleTag(tagStr string, vars map[string]string, condStack *[]condState, out *strings.Builder, emit bool) error {
	switch {
	case strings.HasPrefix(tagStr, "if:"):
		varName := strings.TrimSpace(strings.TrimPrefix(tagStr, "if:"))
		if emit {
			val, ok := vars[varName]
			active := ok && val != ""
			*condStack = append(*condStack, condState{anyTaken: active, current: active})
		} else {
			*condStack = append(*condStack, condState{anyTaken: true, current: false})
		}

	case strings.HasPrefix(tagStr, "elseif:"):
		if len(*condStack) == 0 {
			return fmt.Errorf("<%selseif:...%%> without matching <%sif:...%%>", "%", "%")
		}
		top := &(*condStack)[len(*condStack)-1]
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
		if len(*condStack) == 0 {
			return fmt.Errorf("<%selse%%> without matching <%sif:...%%>", "%", "%")
		}
		top := &(*condStack)[len(*condStack)-1]
		if top.anyTaken {
			top.current = false
		} else {
			top.current = true
			top.anyTaken = true
		}

	case tagStr == "endif":
		if len(*condStack) == 0 {
			return fmt.Errorf("<%sendif%%> without matching <%sif:...%%>", "%", "%")
		}
		*condStack = (*condStack)[:len(*condStack)-1]

	default:
		if emit {
			resolved, err := ms.resolveSingleTag(tagStr, vars)
			if err != nil {
				return err
			}
			out.WriteString(resolved)
		}
	}
	return nil
}

// ---

func (ms *Miniskin) resolveSingleTag(name string, vars map[string]string) (string, error) {
	val, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("undefined variable <%%%s%%>", name)
	}
	return html.EscapeString(val), nil
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
