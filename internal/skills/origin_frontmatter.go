package skills

import "strings"

func frontmatterOrigin(front frontmatter) Origin {
	repository := firstTopValue(front, []string{"repository", "repo", "source"})
	if repository == "" {
		repository = front.nested["repository"]["url"]
	}
	if repository == "" {
		repository = firstMetadataValue(front, []string{"repository", "repo", "source"})
	}
	homepage := firstTopValue(front, []string{"homepage", "url", "website"})
	if homepage == "" {
		homepage = firstMetadataValue(front, []string{"homepage", "url", "website"})
	}
	version := strings.TrimSpace(front.values["version"])
	if version == "" {
		version = strings.TrimSpace(front.metadata["version"])
	}
	candidate := originCandidate{from: OriginFrontmatter, homepage: normalizeHomepage(homepage), version: version}
	if repo, _, ok := NormalizeRepoURL(repository); ok {
		candidate.repo = repo
	}
	if candidate.repo == "" && candidate.homepage == "" && candidate.version == "" {
		return Origin{}
	}
	return Origin{From: OriginFrontmatter, Repository: candidate.repo, Homepage: candidate.homepage, Version: candidate.version}
}

func firstTopValue(front frontmatter, keys []string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(front.values[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstMetadataValue(front frontmatter, keys []string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(front.metadata[key]); value != "" {
			return value
		}
	}
	return ""
}
