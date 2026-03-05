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

type pctTag struct {
	double bool
	name   string
}

// resolvePercent processes <%var%> (escaped) and <%%var%%> (literal) tags
// in a single pass using a finite state machine.
func (ms *Miniskin) resolvePercent(content string, vars map[string]string, chain []string) (string, error) {
	var out strings.Builder
	var tag strings.Builder
	state := stText

	for i := 0; i < len(content); i++ {
		c := content[i]

		switch state {
		case stText:
			if c == '<' {
				state = stLT
			} else {
				out.WriteByte(c)
			}

		case stLT:
			if c == '%' {
				state = stLTPct
			} else {
				out.WriteByte('<')
				out.WriteByte(c)
				state = stText
			}

		case stLTPct:
			if c == '%' {
				state = stLTPctPct
			} else {
				// start of single tag <%
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
				// complete single tag <%...%>
				resolved, err := ms.resolveSingleTag(tag.String(), vars)
				if err != nil {
					return "", err
				}
				out.WriteString(resolved)
				state = stText
			} else {
				// false alarm, the % was part of the tag content
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stSingle
			}

		case stLTPctPct:
			// start of double tag <%%
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
				// false alarm, single % inside double tag
				tag.WriteByte('%')
				tag.WriteByte(c)
				state = stDouble
			}

		case stDoubleClose2:
			if c == '>' {
				// complete double tag <%%...%%>
				resolved, err := ms.resolveDoubleTag(tag.String(), vars, chain)
				if err != nil {
					return "", err
				}
				out.WriteString(resolved)
				state = stText
			} else {
				// false alarm, %% was part of the tag content
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
		out.WriteByte('<')
	case stLTPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stSingle, stSingleClose:
		return "", fmt.Errorf("unclosed <%% tag: <%s", tag.String())
	case stLTPctPct:
		return "", fmt.Errorf("unclosed <%% tag at end of content")
	case stDouble, stDoubleClose1, stDoubleClose2:
		return "", fmt.Errorf("unclosed <%%%% tag: <%%%s", tag.String())
	}

	return out.String(), nil
}

// ---

func (ms *Miniskin) resolveSingleTag(name string, vars map[string]string) (string, error) {
	name = strings.TrimSpace(name)
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
