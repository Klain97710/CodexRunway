package skills

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errNotRegular = errors.New("skills: skill file is not a regular file")
var errNotSkillFile = errors.New("skills: path is not a SKILL.md file")

// ReadBody returns the Markdown after frontmatter up to MaxBodyBytes.
func ReadBody(skill Skill) (string, bool, error) {
	if err := validateSkillPath(skill.File); err != nil {
		return "", false, err
	}
	file, err := openRegular(skill.File)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	prefix, err := io.ReadAll(io.LimitReader(file, MaxFrontmatterBytes+1))
	if err != nil {
		return "", false, err
	}
	frontPrefix := prefix
	if len(frontPrefix) > MaxFrontmatterBytes {
		frontPrefix = frontPrefix[:MaxFrontmatterBytes]
	}
	start, hasFront, err := frontmatterEnd(frontPrefix)
	if err != nil {
		return "", false, err
	}
	if !hasFront {
		start = 0
	}
	if _, err := file.Seek(int64(start), io.SeekStart); err != nil {
		return "", false, err
	}
	body, err := io.ReadAll(io.LimitReader(file, MaxBodyBytes+1))
	if err != nil {
		return "", false, err
	}
	truncated := len(body) > MaxBodyBytes
	if truncated {
		body = body[:MaxBodyBytes]
	}
	return string(body), truncated, nil
}

func validateSkillPath(path string) error {
	if filepath.Base(path) != "SKILL.md" {
		return errNotSkillFile
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if filepath.Base(real) != "SKILL.md" {
		return errNotSkillFile
	}
	return nil
}

func openRegular(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errNotRegular
	}
	return os.Open(path)
}

func firstParagraph(body string) string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if len(lines) == 0 && (strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---")) {
			continue
		}
		lines = append(lines, line)
	}
	return singleLine(strings.Join(lines, " "))
}

func frontmatterPrefix(file *os.File) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, MaxFrontmatterBytes+1))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func frontmatterEnd(data []byte) (int, bool, error) {
	lines, offsets := splitLines(data)
	if len(lines) == 0 || lines[0] != "---" {
		return 0, false, nil
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] != "---" {
			continue
		}
		start := offsets[i] + len(lines[i])
		if start < len(data) && data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		return start, true, nil
	}
	return 0, true, errInvalidFrontmatter
}
