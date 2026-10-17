package skills

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type cachedClaudeOriginFiles struct {
	installedLoaded bool
	installed       claudeInstalledPlugins
	knownLoaded     bool
	known           map[string]claudeMarketplace
	marketplaces    map[string]cachedClaudeMarketplace
}

type cachedClaudeMarketplace struct {
	manifest claudeMarketplaceManifest
	ok       bool
}

type claudeMarketplace struct {
	Source          json.RawMessage `json:"source"`
	Repo            string          `json:"repo"`
	URL             string          `json:"url"`
	Path            string          `json:"path"`
	InstallLocation string          `json:"installLocation"`
	LastUpdated     string          `json:"lastUpdated"`
}

type claudeMarketplaceManifest struct {
	Plugins []claudeMarketplacePlugin `json:"plugins"`
}

type claudeMarketplacePlugin struct {
	Name       string          `json:"name"`
	Source     json.RawMessage `json:"source"`
	Homepage   string          `json:"homepage"`
	Repository json.RawMessage `json:"repository"`
	Version    string          `json:"version"`
}

type claudePluginSource struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
	URL    string `json:"url"`
	Path   string `json:"path"`
}

func (c *originPluginCache) claudeCandidate(skill Skill, root string, candidate originCandidate) originCandidate {
	installed := c.claudeInstalled()
	root = cleanEvaluatedPath(root)
	for pluginID, installs := range installed.Plugins {
		name, market, _ := strings.Cut(pluginID, "@")
		if name != skill.Plugin {
			continue
		}
		for _, install := range installs {
			if install.Scope != "user" || strings.TrimSpace(install.InstallPath) == "" || !filepath.IsAbs(install.InstallPath) {
				continue
			}
			if cleanEvaluatedPath(install.InstallPath) != root {
				continue
			}
			candidate.from = OriginPlugin
			candidate.installer = market
			if candidate.version == "" && strings.TrimSpace(install.Version) != "" {
				candidate.version = strings.TrimSpace(install.Version)
			}
			if candidate.ref == "" {
				candidate.ref = strings.TrimSpace(install.GitCommitSHA)
			}
			candidate.installedAt = parseOriginTime(install.InstalledAt)
			candidate.updatedAt = parseOriginTime(install.LastUpdated)
			if candidate.repo != "" {
				return candidate
			}
			return c.claudeMarketplaceCandidate(skill, root, market, candidate)
		}
	}
	return candidate
}

func (c *originPluginCache) claudeInstalled() claudeInstalledPlugins {
	if c.claude.installedLoaded {
		return c.claude.installed
	}
	c.claude.installedLoaded = true
	path := filepath.Join(c.env.ClaudeDir, "plugins", "installed_plugins.json")
	data, err := readOriginManifest(path)
	if err == nil && json.Unmarshal(data, &c.claude.installed) == nil && c.claude.installed.Version == 2 {
		return c.claude.installed
	}
	c.claude.installed = claudeInstalledPlugins{}
	return c.claude.installed
}

func (c *originPluginCache) claudeMarketplaceCandidate(skill Skill, root, market string, candidate originCandidate) originCandidate {
	known, ok := c.claudeKnownMarketplaces()[market]
	if !ok {
		return candidate
	}
	marketRepo, _, _ := sourceRepository(known.sourceValue())
	if repo, ref, _ := repositoryParts(marketRepo); repo != "" {
		candidate.repo = repo
		if ref != "" || candidate.ref == "" {
			candidate.ref = ref
		}
	}
	if strings.TrimSpace(known.InstallLocation) == "" || !filepath.IsAbs(known.InstallLocation) {
		return candidate
	}
	manifest, ok := c.claudeMarketplaceManifest(filepath.Join(known.InstallLocation, ".claude-plugin", "marketplace.json"))
	if !ok {
		return candidate
	}
	for _, plugin := range manifest.Plugins {
		if strings.TrimSpace(plugin.Name) != skill.Plugin {
			continue
		}
		if candidate.homepage == "" {
			candidate.homepage = normalizeHomepage(plugin.Homepage)
		}
		if candidate.version == "" {
			candidate.version = strings.TrimSpace(plugin.Version)
		}
		pluginRepo := repositoryValue(plugin.Repository)
		repo, ref, prefix := repositoryParts(pluginRepo)
		if sourceRepo, sourcePath, relative := sourceRepository(plugin.Source); sourceRepo != "" || relative {
			if sourceRepo != "" {
				repo, ref, prefix = repositoryParts(sourceRepo)
			}
			if relative {
				repo, ref, _ = repositoryParts(marketRepo)
				prefix = sourcePath
			} else if sourcePath != "" {
				prefix = sourcePath
			} else {
				prefix = ""
			}
		}
		if repo != "" {
			candidate.repo = repo
			if ref != "" || candidate.ref == "" {
				candidate.ref = ref
			}
		}
		if candidate.repo != "" && (prefix != "" || isRelativePluginSource(plugin.Source)) {
			candidate.skillPath = joinOriginPath(prefix, relativeOriginPath(root, cleanEvaluatedPath(filepath.Dir(skill.RealFile))))
		}
		return candidate
	}
	return candidate
}

func (market claudeMarketplace) sourceValue() json.RawMessage {
	var kind string
	if json.Unmarshal(market.Source, &kind) != nil {
		return market.Source
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "github", "git", "url", "directory":
	default:
		return market.Source
	}
	data, _ := json.Marshal(claudePluginSource{Source: kind, Repo: market.Repo, URL: market.URL, Path: market.Path})
	return data
}

func (c *originPluginCache) claudeKnownMarketplaces() map[string]claudeMarketplace {
	if c.claude.knownLoaded {
		return c.claude.known
	}
	c.claude.knownLoaded = true
	c.claude.known = make(map[string]claudeMarketplace)
	path := filepath.Join(c.env.ClaudeDir, "plugins", "known_marketplaces.json")
	data, err := readOriginManifest(path)
	if err != nil {
		return c.claude.known
	}
	if json.Unmarshal(data, &c.claude.known) == nil {
		return c.claude.known
	}
	c.claude.known = make(map[string]claudeMarketplace)
	var wrapped struct {
		Marketplaces map[string]claudeMarketplace `json:"marketplaces"`
	}
	if json.Unmarshal(data, &wrapped) == nil && wrapped.Marketplaces != nil {
		c.claude.known = wrapped.Marketplaces
	}
	return c.claude.known
}

func (c *originPluginCache) claudeMarketplaceManifest(path string) (claudeMarketplaceManifest, bool) {
	if c.claude.marketplaces == nil {
		c.claude.marketplaces = make(map[string]cachedClaudeMarketplace)
	}
	path = filepath.Clean(path)
	if cached, ok := c.claude.marketplaces[path]; ok {
		return cached.manifest, cached.ok
	}
	var manifest claudeMarketplaceManifest
	data, err := readOriginManifest(path)
	ok := err == nil && json.Unmarshal(data, &manifest) == nil
	c.claude.marketplaces[path] = cachedClaudeMarketplace{manifest: manifest, ok: ok}
	return manifest, ok
}

func sourceRepository(raw json.RawMessage) (repo, path string, relative bool) {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", "", false
		}
		if normalized, _, ok := NormalizeRepoURL(value); ok {
			return normalized, "", false
		}
		if filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, "../") || strings.Contains("/"+value+"/", "/../") {
			return "", "", false
		}
		return "", cleanOriginSkillPath(strings.TrimPrefix(value, "./")), true
	}
	var source claudePluginSource
	if json.Unmarshal(raw, &source) != nil {
		return "", "", false
	}
	switch strings.ToLower(strings.TrimSpace(source.Source)) {
	case "github":
		if strings.Count(strings.Trim(source.Repo, "/"), "/") != 1 {
			return "", "", false
		}
		value = "https://github.com/" + strings.Trim(source.Repo, "/")
	case "git", "url":
		value = source.URL
		if value == "" {
			value = source.Repo
		}
	default:
		return "", "", false
	}
	return strings.TrimSpace(value), cleanOriginSkillPath(source.Path), false
}

func isRelativePluginSource(raw json.RawMessage) bool {
	_, _, relative := sourceRepository(raw)
	return relative
}

func repositoryParts(raw string) (repo, ref, path string) {
	if parsed, parsedRef, parsedPath := parseRepoTreeURL(raw); parsed != "" {
		repo, _, _ = NormalizeRepoURL(parsed)
		return repo, parsedRef, parsedPath
	}
	repo, _, _ = NormalizeRepoURL(raw)
	return repo, "", ""
}
