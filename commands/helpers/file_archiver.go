package helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

type fileArchiver struct {
	Paths     []string `long:"path" description:"Add paths to archive"`
	Exclude   []string `long:"exclude" description:"Exclude paths from the archive"`
	Untracked bool     `long:"untracked" description:"Add git untracked files"`
	Verbose   bool     `long:"verbose" description:"Detailed information"`

	wd    string
	files map[string]os.FileInfo
}

func (c *fileArchiver) isChanged(modTime time.Time) bool {
	for _, info := range c.files {
		if modTime.Before(info.ModTime()) {
			return true
		}
	}
	return false
}

func (c *fileArchiver) isFileChanged(fileName string) bool {
	ai, err := os.Stat(fileName)
	if ai != nil {
		if !c.isChanged(ai.ModTime()) {
			return false
		}
	} else if !os.IsNotExist(err) {
		logrus.Warningln(err)
	}
	return true
}

func (c *fileArchiver) sortedFiles() []string {
	files := make([]string, len(c.files))

	i := 0
	for file := range c.files {
		files[i] = file
		i++
	}

	sort.Strings(files)
	return files
}

func (c *fileArchiver) enumerate() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	c.wd = wd

	includes := deprecated14PatternCheck(wd, c.Paths)
	excludes := deprecated14PatternCheck(wd, c.Exclude)

	files, stats, err := match(context.Background(), wd, includes, excludes, c.Untracked)
	if err != nil {
		return err
	}

	if c.Untracked {
		logrus.Infof(
			"found %d files and directories (%d matched, %d untracked, %s) in %v",
			stats.Files+stats.Untracked,
			stats.Files,
			stats.Untracked,
			units.HumanSize(float64(stats.TotalSize)),
			stats.Duration,
		)
	} else {
		logrus.Infof(
			"found %d files and directories (%s) in %v",
			stats.Files,
			units.HumanSize(float64(stats.TotalSize)),
			stats.Duration,
		)
	}

	// convert absolute path to relative, to bridge with existing code
	// this can eventually be removed once all archivers handle this themselves
	c.convertToRelativePaths(files)

	return nil
}

func (c *fileArchiver) convertToRelativePaths(files map[string]os.FileInfo) {
	dir := filepath.ToSlash(c.wd)

	c.files = make(map[string]os.FileInfo, len(files))
	for path, fi := range files {
		path = filepath.ToSlash(path)
		path = strings.TrimPrefix(path, dir)
		path = strings.TrimPrefix(path, "/")

		c.files[path] = fi
	}
}
