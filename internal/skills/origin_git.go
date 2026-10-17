package skills

import (
	"os"
	"path/filepath"
	"strings"
)

type originGitCache struct {
	env     Env
	repos   map[string]originGitResult
	markers map[string]originGitCheck
}

type originGitCheck struct {
	present bool
	repo    originGitRepo
}

type originGitResult struct {
	found bool
	repo  originGitRepo
}

type originGitRepo struct {
	root, remote, branch string
}

func newOriginGitCache(env Env) *originGitCache {
	return &originGitCache{env: env, repos: make(map[string]originGitResult), markers: make(map[string]originGitCheck)}
}

func (c *originGitCache) candidate(skill Skill) originCandidate {
	start := filepath.Dir(skill.RealFile)
	if start == "." || start == "" {
		return originCandidate{}
	}
	start, err := filepath.EvalSymlinks(start)
	if err != nil {
		return originCandidate{}
	}
	boundary := c.boundary(skill, start)
	repo, ok := c.find(start, boundary)
	if !ok {
		return originCandidate{}
	}
	remote, _, ok := NormalizeRepoURL(repo.remote)
	if !ok {
		return originCandidate{}
	}
	rel, err := filepath.Rel(repo.root, start)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return originCandidate{}
	}
	skillPath := ""
	if rel != "." {
		skillPath = cleanOriginSkillPath(filepath.ToSlash(rel))
	}
	return originCandidate{from: OriginGit, repo: remote, ref: repo.branch, branch: repo.branch, skillPath: skillPath, installer: "git"}
}

type originGitBoundary struct {
	key       string
	stop      string
	include   bool
	maxLevels int
}

func (c *originGitCache) boundary(skill Skill, start string) originGitBoundary {
	if root, err := filepath.EvalSymlinks(skill.Root); err == nil && within(root, start) {
		root = filepath.Clean(root)
		return originGitBoundary{key: "root:" + root, stop: root, include: true}
	}
	home := c.env.Home
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	home = filepath.Clean(home)
	if within(home, start) {
		return originGitBoundary{key: "home:" + home, stop: home, include: false}
	}
	return originGitBoundary{key: "external:8:" + start, maxLevels: 8}
}

func (c *originGitCache) find(start string, boundary originGitBoundary) (originGitRepo, bool) {
	var visited []string
	dir := filepath.Clean(start)
	for level := 0; ; level++ {
		if boundary.maxLevels > 0 && level >= boundary.maxLevels {
			break
		}
		if dir == boundary.stop && !boundary.include {
			break
		}
		if boundary.maxLevels > 0 && filepath.Dir(dir) == dir {
			break
		}
		key := boundary.key + "\x00" + dir
		if cached, ok := c.repos[key]; ok {
			c.cacheVisited(boundary.key, visited, cached)
			return cached.repo, cached.found
		}
		visited = append(visited, dir)
		check := c.marker(dir)
		if check.present {
			repo := check.repo
			result := originGitResult{found: repo.remote != "", repo: repo}
			c.cacheVisited(boundary.key, visited, result)
			return result.repo, result.found
		}
		if dir == boundary.stop || filepath.Dir(dir) == dir {
			break
		}
		dir = filepath.Dir(dir)
	}
	c.cacheVisited(boundary.key, visited, originGitResult{})
	return originGitRepo{}, false
}

func (c *originGitCache) marker(dir string) originGitCheck {
	if check, ok := c.markers[dir]; ok {
		return check
	}
	repo, present := readOriginGitRepo(dir)
	check := originGitCheck{present: present, repo: repo}
	c.markers[dir] = check
	return check
}

func (c *originGitCache) cacheVisited(prefix string, dirs []string, result originGitResult) {
	for _, dir := range dirs {
		c.repos[prefix+"\x00"+dir] = result
	}
}

func readOriginGitRepo(root string) (originGitRepo, bool) {
	marker := filepath.Join(root, ".git")
	info, err := os.Lstat(marker)
	if err != nil {
		return originGitRepo{}, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return originGitRepo{root: root}, true
	}
	gitDir, ok := originGitDir(root, marker, info)
	if !ok {
		return originGitRepo{root: root}, true
	}
	commonDir, ok := originGitCommonDir(gitDir)
	if !ok {
		return originGitRepo{root: root}, true
	}
	config, err := readOriginRegular(filepath.Join(commonDir, "config"))
	if err != nil {
		return originGitRepo{root: root}, true
	}
	remote := originRemoteURL(string(config))
	branch := ""
	if head, err := readOriginRegular(filepath.Join(gitDir, "HEAD")); err == nil {
		const prefix = "ref: refs/heads/"
		value := strings.TrimSpace(string(head))
		if strings.HasPrefix(value, prefix) {
			branch = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return originGitRepo{root: root, remote: remote, branch: branch}, true
}

func originGitDir(root, marker string, info os.FileInfo) (string, bool) {
	gitDir := marker
	if info.Mode().IsRegular() {
		data, err := readOriginRegular(marker)
		if err != nil {
			return "", false
		}
		line := strings.TrimSpace(string(data))
		if len(line) < len("gitdir:") || !strings.EqualFold(line[:len("gitdir:")], "gitdir:") {
			return "", false
		}
		gitDir = strings.TrimSpace(line[len("gitdir:"):])
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(root, gitDir)
		}
	} else if !info.IsDir() {
		return "", false
	}
	gitDir = filepath.Clean(gitDir)
	gitInfo, err := os.Lstat(gitDir)
	return gitDir, err == nil && gitInfo.IsDir() && gitInfo.Mode()&os.ModeSymlink == 0
}

func originGitCommonDir(gitDir string) (string, bool) {
	data, err := readOriginRegular(filepath.Join(gitDir, "commondir"))
	if os.IsNotExist(err) {
		return gitDir, true
	}
	if err != nil {
		return "", false
	}
	commonDir := strings.TrimSpace(string(data))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}
	return filepath.Clean(commonDir), true
}

func originRemoteURL(config string) string {
	section, first, origin := "", "", ""
	for _, raw := range strings.Split(strings.ReplaceAll(config, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if !strings.HasPrefix(section, "remote ") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "url") {
			continue
		}
		value = trimOriginINIValue(value)
		if first == "" {
			first = value
		}
		name := strings.Trim(strings.TrimSpace(strings.TrimPrefix(section, "remote ")), "\"'")
		if name == "origin" && origin == "" {
			origin = value
		}
	}
	if origin != "" {
		return origin
	}
	return first
}

func trimOriginINIValue(value string) string {
	value = strings.TrimSpace(value)
	quote := byte(0)
	for i := 0; i < len(value); i++ {
		if quote != 0 {
			if value[i] == quote {
				quote = 0
			}
			continue
		}
		if value[i] == '"' || value[i] == '\'' {
			quote = value[i]
			continue
		}
		if (value[i] == '#' || value[i] == ';') && i > 0 && (value[i-1] == ' ' || value[i-1] == '\t') {
			value = strings.TrimSpace(value[:i])
			break
		}
	}
	if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
