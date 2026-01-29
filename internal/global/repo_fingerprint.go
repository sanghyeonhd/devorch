// Package global provides repository fingerprinting.
// Phase 33: Repo fingerprint (git state, language detection)
package global

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoFingerprint represents a repository's identity.
type RepoFingerprint struct {
	Root        string
	HeadCommit  string
	Dirty       bool
	PrimaryLang string
}

// DetectRepoFingerprint detects repository fingerprint from the given root.
func DetectRepoFingerprint(repoRoot string) RepoFingerprint {
	r := RepoFingerprint{Root: repoRoot}

	r.HeadCommit = gitCommand(repoRoot, "rev-parse", "HEAD")
	if r.HeadCommit == "" {
		r.HeadCommit = "nogit"
	}

	status := gitCommand(repoRoot, "status", "--porcelain")
	r.Dirty = strings.TrimSpace(status) != ""

	r.PrimaryLang = detectPrimaryLang(repoRoot)

	return r
}

// Hash returns a stable hash of the repo fingerprint.
func (r RepoFingerprint) Hash() string {
	s := r.HeadCommit + "|" + boolToStr(r.Dirty) + "|" + r.PrimaryLang
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ShortHash returns a shortened version of the hash.
func (r RepoFingerprint) ShortHash() string {
	h := r.Hash()
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func gitCommand(root string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectPrimaryLang(root string) string {
	checks := []struct {
		file string
		lang string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"package.json", "javascript"},
		{"tsconfig.json", "typescript"},
		{"pyproject.toml", "python"},
		{"requirements.txt", "python"},
		{"setup.py", "python"},
		{"build.gradle", "java"},
		{"build.gradle.kts", "kotlin"},
		{"pom.xml", "java"},
		{"Gemfile", "ruby"},
		{"composer.json", "php"},
		{"CMakeLists.txt", "cpp"},
		{"Makefile", "c"},
	}

	for _, c := range checks {
		if fileExists(filepath.Join(root, c.file)) {
			return c.lang
		}
	}

	return "unknown"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
