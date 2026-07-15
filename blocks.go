package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var blockStart = regexp.MustCompile(`^\s*%\s*@resume-block\s+id=([^\s]+)(?:\s+type=([^\s]+))?(?:\s+title="([^"]*)")?\s*$`)
var blockEnd = regexp.MustCompile(`^\s*%\s*@end-resume-block\s*$`)

func parseResumeBlocks(source, file string) ([]ResumeBlock, error) {
	scanner := bufio.NewScanner(strings.NewReader(source))
	scanner.Buffer(make([]byte, 1024), 1024*1024*4)
	line, offset := 0, 0
	var open *ResumeBlock
	seen := map[string]bool{}
	var blocks []ResumeBlock
	for scanner.Scan() {
		line++
		text := scanner.Text()
		lineBytes := len(text)
		newlineBytes := 1
		if offset+lineBytes < len(source) && source[offset+lineBytes] == '\r' {
			newlineBytes = 2
		}
		if match := blockStart.FindStringSubmatch(text); match != nil {
			if open != nil {
				return nil, fmt.Errorf("第 %d 行出现嵌套 resume-block", line)
			}
			if seen[match[1]] {
				return nil, fmt.Errorf("重复的 resume-block id: %s", match[1])
			}
			title := match[3]
			if title == "" {
				title = humanizeID(match[1])
			}
			typ := match[2]
			if typ == "" {
				typ = "section"
			}
			open = &ResumeBlock{ID: match[1], Type: typ, Title: title, File: file, StartLine: line + 1, startOffset: offset + lineBytes + newlineBytes}
			seen[match[1]] = true
		} else if blockEnd.MatchString(text) {
			if open == nil {
				return nil, fmt.Errorf("第 %d 行存在未匹配的 @end-resume-block", line)
			}
			open.EndLine = line - 1
			open.endOffset = offset
			end := open.endOffset
			for end > open.startOffset && (source[end-1] == '\n' || source[end-1] == '\r') {
				end--
			}
			open.Content = source[open.startOffset:end]
			open.endOffset = end
			blocks = append(blocks, *open)
			open = nil
		}
		offset += lineBytes + newlineBytes
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if open != nil {
		return nil, fmt.Errorf("resume-block %s 缺少结束标记", open.ID)
	}
	return blocks, nil
}

func humanizeID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, " / ")
}
