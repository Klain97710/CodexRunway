package skills

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func loadSkill(env Env, provider Provider, root RootInfo, file string) (Skill, []Issue, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return Skill{}, nil, err
	}
	file = filepath.Clean(abs)
	if err := validateSkillPath(file); err != nil {
		return Skill{}, nil, err
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return Skill{}, nil, err
	}
	info, err := os.Stat(file)
	if err != nil {
		return Skill{}, nil, err
	}
	if !info.Mode().IsRegular() {
		return Skill{}, nil, errNotRegular
	}
	skill := Skill{
		ID: string(provider) + "\x00" + file, Provider: provider,
		Name: filepath.Base(filepath.Dir(file)), InvokeName: filepath.Base(filepath.Dir(file)),
		File: file, Dir: filepath.Dir(file), RealFile: filepath.Clean(real),
		Scope: root.Scope, Root: root.Path, RootLabel: root.Label,
		State: StateOn, ModTime: info.ModTime(),
	}
	skill.LinkPath, skill.LinkTarget = firstSymlink(root.Path, file)
	front, err := readSkillFront(file)
	if err != nil {
		if !errors.Is(err, errInvalidFrontmatter) {
			return Skill{}, nil, err
		}
		skill.State, skill.Reason = StateNotLoaded, ReasonInvalidFront
		return skill, []Issue{{Kind: IssueInvalidFront, Path: file, Name: skill.Name}}, nil
	}
	applyFrontmatter(&skill, front)
	return applyDescriptionRule(skill)
}

func readSkillFront(file string) (frontmatter, error) {
	handle, err := openRegular(file)
	if err != nil {
		return frontmatter{}, err
	}
	prefix, err := frontmatterPrefix(handle)
	handle.Close()
	if err != nil {
		return frontmatter{}, err
	}
	if len(prefix) > MaxFrontmatterBytes {
		prefix = prefix[:MaxFrontmatterBytes]
	}
	front, _, err := parseFrontmatter(prefix)
	return front, err
}

func applyFrontmatter(skill *Skill, front frontmatter) {
	if name := front.field("name"); name != "" {
		skill.Name, skill.InvokeName = name, name
	}
	skill.Description = front.field("description")
	skill.ShortDescription = front.metadata["short-description"]
	skill.Fields = front.displayFields()
	if skill.Provider == ProviderCodex {
		loadCodexInterface(skill)
	}
}

func applyDescriptionRule(skill Skill) (Skill, []Issue, error) {
	if skill.Description != "" {
		return skill, nil, nil
	}
	issue := Issue{Kind: IssueMissingDesc, Path: skill.File, Name: skill.Name}
	switch skill.Provider {
	case ProviderCodex:
		skill.State, skill.Reason = StateNotLoaded, ReasonMissingDesc
		return skill, []Issue{issue}, nil
	case ProviderGrok:
		body, _, err := ReadBody(skill)
		if err != nil {
			return Skill{}, nil, err
		}
		skill.Description = firstParagraph(body)
		if skill.Description == "" {
			return skill, []Issue{issue}, nil
		}
		return skill, nil, nil
	default:
		return skill, []Issue{issue}, nil
	}
}

func firstSymlink(root, file string) (string, string) {
	root = filepath.Clean(root)
	if !within(root, file) {
		return "", ""
	}
	candidates := []string{root}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return "", ""
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		candidates = append(candidates, current)
	}
	for _, path := range candidates {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", ""
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target, err = filepath.Abs(target)
		if err != nil {
			return "", ""
		}
		return path, filepath.Clean(target)
	}
	return "", ""
}

func loadCodexInterface(skill *Skill) {
	data, err := readSmallRegular(filepath.Join(skill.Dir, "agents", "openai.yaml"), MaxFrontmatterBytes)
	if err != nil {
		return
	}
	wrapper := append([]byte("---\n"), data...)
	wrapper = append(wrapper, []byte("\n---\n")...)
	front, _, err := parseFrontmatter(wrapper)
	if err != nil {
		return
	}
	interfaceValues := front.nested["interface"]
	policyValues := front.nested["policy"]
	policy, hasPolicy := policyValues["allow_implicit_invocation"]
	if hasPolicy {
		if _, err := strconv.ParseBool(policy); err != nil {
			return
		}
	}
	if value, ok := interfaceValues["display_name"]; ok {
		skill.DisplayName = value
	}
	if value, ok := interfaceValues["short_description"]; ok {
		skill.ShortDescription = value
	}
	if hasPolicy {
		skill.Fields = append(skill.Fields, Field{Key: "policy.allow_implicit_invocation", Value: policy})
	}
}

func readSmallRegular(path string, limit int64) ([]byte, error) {
	file, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > int(limit) {
		return nil, errInvalidFrontmatter
	}
	return data, nil
}
