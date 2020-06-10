package helpers

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar"
	"github.com/saracen/matcher"
	"github.com/sirupsen/logrus"
)

type excludeMatcher struct {
	excludes matcher.Matcher
	includes matcher.Matcher
}

func (m *excludeMatcher) Match(pathname string) (matcher.Result, error) {
	result, err := m.excludes.Match(pathname)
	if result == matcher.Matched || err != nil {
		return matcher.NotMatched, err
	}

	return m.includes.Match(pathname)
}

type matchStats struct {
	Files     int
	Untracked int
	Duration  time.Duration
	TotalSize int64
}

func createMatcher(patterns []string) matcher.Matcher {
	var matchers []matcher.Matcher

	// matcher supports globstar (**) out of the box, however, doublestar.Match
	// also provides additional patterns that path.Match (the default match
	// function) does not.
	withExtendedPattern := matcher.WithMatchFunc(doublestar.Match)

	for _, p := range patterns {
		// rewrite pattern to conform to previous or intended behaviour
		switch p {
		case `*`, `.`, `./`:
			matchers = append(matchers, matcher.New("**/*"))
			continue
		}

		if !strings.HasSuffix(p, "/") {
			matchers = append(matchers, matcher.New(p, withExtendedPattern))
		}

		// add recursive match
		matchers = append(matchers, matcher.New(path.Join(p, "**"), withExtendedPattern))
	}

	return matcher.Multi(matchers...)
}

func match(
	ctx context.Context,
	dir string,
	includes, excludes []string,
	untracked bool,
) (map[string]os.FileInfo, *matchStats, error) {
	start := time.Now()

	files, err := matcher.Glob(ctx, dir, &excludeMatcher{
		excludes: createMatcher(excludes),
		includes: createMatcher(includes),
	})
	if err != nil {
		return files, nil, err
	}

	stats := &matchStats{
		Files: len(files),
	}

	if untracked {
		stats.Untracked, err = processGitUntracked(ctx, dir, files)
		if err != nil {
			return files, stats, err
		}
	}

	for _, fi := range files {
		stats.TotalSize += fi.Size()
	}

	stats.Duration = time.Since(start)

	return files, stats, err
}

func processGitUntracked(ctx context.Context, dir string, files map[string]os.FileInfo) (int, error) {
	buf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-o", "-z")
	cmd.Dir = dir
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		return 0, err
	}

	var count int
	for _, path := range strings.Split(buf.String(), "\x00") {
		if path == "" {
			continue
		}

		path = filepath.Join(dir, path)
		if _, ok := files[path]; ok {
			continue
		}

		fi, err := os.Lstat(path)
		if err != nil {
			continue
		}

		files[path] = fi
		count++
	}

	return count, nil
}

func deprecated14PatternCheck(dir string, patterns []string) []string {
	normalized := make(map[string]string, len(patterns))

	for _, pattern := range patterns {
		slashed := filepath.ToSlash(pattern)

		if slashed != pattern {
			logrus.Warningf("pattern %q contains backslashes: from GitLab v14.0, "+
				"backslashes will be solely reserved for pattern escape characters.", pattern)

			logrus.Infof("converted pattern %q: %q", pattern, slashed)
		}

		// ToSlash only modifies the pattern on windows, so we don't have to
		// worry about it modifying legit patterns on other operating systems.
		normalized[pattern] = slashed

		if !filepath.IsAbs(pattern) {
			continue
		}

		logrus.Warningf("pattern %q is an absolute path, from GitLab v14.0, "+
			"absolute paths for patterns will be unsupported", pattern)

		rel, err := filepath.Rel(dir, pattern)
		rel = filepath.ToSlash(rel)

		switch {
		// pattern is a child path to build directory
		case err == nil && !strings.Contains(rel, ".."):
			normalized[pattern] = rel

			logrus.Infof("converted pattern %q: %q", pattern, rel)

		// if the relative path is *only* ../ sequences, we can convert the pattern
		// to be a recursive match for the build directory, as we know the original
		// path supplied was outside of the build directory, but also a parent to it
		case err == nil && strings.Trim(rel, "./") == "":
			normalized[pattern] = "./"

			logrus.Infof("converted pattern %q: ./", pattern)

		default:
			logrus.Warningf("pattern path %q is outside of the build directory and will be ignored", pattern)

			delete(normalized, pattern)
		}
	}

	paths := make([]string, 0, len(normalized))
	for _, pattern := range patterns {
		path, ok := normalized[pattern]
		if ok {
			paths = append(paths, path)
		}
	}

	return paths
}
