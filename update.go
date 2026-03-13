package miniskin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tagInfo records a percent-tag's position and content within a string.
type tagInfo struct {
	start   int    // byte offset of the opening '<'
	end     int    // byte offset after the closing '>'
	content string // tag body between delimiters
}

// UpdateImports walks all mockup source files and refreshes the inline content
// of mockup-import blocks. Single import tags are promoted to block tags
// (import + content + end). Existing blocks get their content replaced.
func (ms *Miniskin) UpdateImports() error {
	_, bl, err := ms.init()
	if err != nil {
		return err
	}

	ms.logf("=== UpdateImports ===")
	for _, bucket := range bl.Buckets {
		if err := ms.walkBucket(bucket, func(parsed *xmlMiniskin, dir string) error {
			if parsed.MockupList == nil {
				return nil
			}
			for _, mi := range parsed.MockupList.Items {
				srcPath := filepath.Join(dir, mi.Src)
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", srcPath, err)
				}

				_, imports := scanExportsImports(string(data))
				if len(imports) == 0 {
					continue
				}

				updated, err := refreshImports(string(data), ms.contentPath)
				if err != nil {
					return fmt.Errorf("updating %s: %w", mi.Src, err)
				}

				if updated != string(data) {
					if err := os.WriteFile(srcPath, []byte(updated), 0644); err != nil {
						return fmt.Errorf("writing %s: %w", srcPath, err)
					}
					ms.logf("  updated: %s", mi.Src)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// refreshImports replaces the inline content of mockup-import blocks
// with the current content of the referenced files.
//
// Single tags:  <!--%%mockup-import:/path%%-->
//            →  <!--%%mockup-import:/path%%-->\ncontent\n<!--%%end%%-->
//
// Block tags:   <!--%%mockup-import:/path%%-->old<!--%%end%%-->
//            →  <!--%%mockup-import:/path%%-->\nnew\n<!--%%end%%-->
func refreshImports(content, contentPath string) (string, error) {
	tags := findTags(content)
	if len(tags) == 0 {
		return content, nil
	}

	var out strings.Builder
	pos := 0

	for i := 0; i < len(tags); {
		trimmed := strings.TrimSpace(tags[i].content)
		filename, ok := isMockupImport(trimmed)
		if !ok {
			i++
			continue
		}

		// Read the referenced file
		filePath := filepath.Join(contentPath, filepath.FromSlash(filename))
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("refreshing mockup-import %s: %w", filename, err)
		}

		// Check if next tag is "end" (existing block)
		hasBlock := false
		if i+1 < len(tags) {
			nextTrimmed := strings.TrimSpace(tags[i+1].content)
			if nextTrimmed == "end" || nextTrimmed == "end-mockup-export" || nextTrimmed == "end-mockup-import" {
				hasBlock = true
			}
		}

		// Emit everything before + the import tag itself
		out.WriteString(content[pos:tags[i].end])
		out.WriteString("\n")
		out.Write(data)

		if hasBlock {
			// Replace old content, preserve the end tag
			out.WriteString("\n")
			out.WriteString(content[tags[i+1].start:tags[i+1].end])
			pos = tags[i+1].end
			i += 2
		} else {
			// Single tag → promote to block
			out.WriteString("\n<!--%%end%%-->")
			pos = tags[i].end
			i++
		}
	}

	if pos < len(content) {
		out.WriteString(content[pos:])
	}

	return out.String(), nil
}

// findTags extracts all percent-tags with their byte positions.
func findTags(content string) []tagInfo {
	var tags []tagInfo
	var tag strings.Builder
	state := stText
	tagStart := 0

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
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
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
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
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
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
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
				tags = append(tags, tagInfo{start: tagStart, end: i + 1, content: tag.String()})
				state = stText
			default:
				tag.WriteString("%%--")
				tag.WriteByte(c)
				state = stCmtTag
			}
		}
	}
	return tags
}
