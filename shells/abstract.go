package shells

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
)

type AbstractShell struct {
}

func (b *AbstractShell) GetFeatures(features *common.FeaturesInfo) {
	features.Artifacts = true
	features.UploadMultipleArtifacts = true
	features.UploadRawArtifacts = true
	features.Cache = true
	features.Refspecs = true
	features.Masking = true
}

func (b *AbstractShell) writeCdBuildDir(w ShellWriter, info common.ShellScriptInfo) {
	w.Cd(info.Build.FullProjectDir())
}

func (b *AbstractShell) cacheFile(build *common.Build, userKey string) (key, file string) {
	if build.CacheDir == "" {
		return
	}

	// Deduce cache key
	key = path.Join(build.JobInfo.Name, build.GitInfo.Ref)
	if userKey != "" {
		key = build.GetAllVariables().ExpandValue(userKey)
	}

	// Ignore cache without the key
	if key == "" {
		return
	}

	file = path.Join(build.CacheDir, key, "cache.zip")
	file, err := filepath.Rel(build.BuildDir, file)
	if err != nil {
		return "", ""
	}
	return
}

func (b *AbstractShell) guardRunnerCommand(w ShellWriter, runnerCommand string, action string, f func()) {
	if runnerCommand == "" {
		w.Warning("%s is not supported by this executor.", action)
		return
	}

	w.IfCmd(runnerCommand, "--version")
	f()
	w.Else()
	w.Warning("Missing %s. %s is disabled.", runnerCommand, action)
	w.EndIf()
}

func (b *AbstractShell) cacheExtractor(w ShellWriter, info common.ShellScriptInfo) error {
	for _, cacheOptions := range info.Build.Cache {

		// Create list of files to extract
		archiverArgs := []string{}
		for _, path := range cacheOptions.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if cacheOptions.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		// Skip restoring cache if no cache is defined
		if len(archiverArgs) < 1 {
			continue
		}

		// Skip extraction if no cache is defined
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Notice("Skipping cache extraction due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPull); err != nil {
			return fmt.Errorf("%s for %s", err, cacheKey)
		} else if !ok {
			w.Notice("Not downloading cache %s due to policy", cacheKey)
			continue
		}

		args := []string{
			"cache-extractor",
			"--file", cacheFile,
			"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
		}

		// Generate cache download address
		if url := cache.GetCacheDownloadURL(info.Build, cacheKey); url != nil {
			args = append(args, "--url", url.String())
		}

		// Execute cache-extractor command. Failure is not fatal.
		b.guardRunnerCommand(w, info.RunnerCommand, "Extracting cache", func() {
			w.Notice("Checking cache for %s...", cacheKey)
			w.IfCmdWithOutput(info.RunnerCommand, args...)
			w.Notice("Successfully extracted cache")
			w.Else()
			w.Warning("Failed to extract cache")
			w.EndIf()
		})
	}

	return nil
}

func (b *AbstractShell) downloadArtifacts(w ShellWriter, job common.Dependency, info common.ShellScriptInfo) {
	args := []string{
		"artifacts-downloader",
		"--url",
		info.Build.Runner.URL,
		"--token",
		job.Token,
		"--id",
		strconv.Itoa(job.ID),
	}

	w.Notice("Downloading artifacts for %s (%d)...", job.Name, job.ID)
	w.Command(info.RunnerCommand, args...)
}

func (b *AbstractShell) jobArtifacts(info common.ShellScriptInfo) (otherJobs []common.Dependency) {
	for _, otherJob := range info.Build.Dependencies {
		if otherJob.ArtifactsFile.Filename == "" {
			continue
		}

		otherJobs = append(otherJobs, otherJob)
	}
	return
}

func (b *AbstractShell) downloadAllArtifacts(w ShellWriter, info common.ShellScriptInfo) {
	otherJobs := b.jobArtifacts(info)
	if len(otherJobs) == 0 {
		return
	}

	b.guardRunnerCommand(w, info.RunnerCommand, "Artifacts downloading", func() {
		for _, otherJob := range otherJobs {
			b.downloadArtifacts(w, otherJob, info)
		}
	})
}

func (b *AbstractShell) writePrepareScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	return nil
}

func (b *AbstractShell) writeGetSourcesScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)

	if !info.Build.IsSharedEnv() {
		b.writeGitSSLConfig(w, info.Build, []string{"--global"})
	}

	if info.PreCloneScript != "" && info.Build.GetGitStrategy() != common.GitNone {
		b.writeCommands(w, info.PreCloneScript)
	}

	return b.writeCloneFetchCmds(w, info)
}

func (b *AbstractShell) writeExports(w ShellWriter, info common.ShellScriptInfo) {
	for _, variable := range info.Build.GetAllVariables() {
		w.Variable(variable)
	}
}

func (b *AbstractShell) writeGitSSLConfig(w ShellWriter, build *common.Build, where []string) {
	repoURL, err := url.Parse(build.Runner.URL)
	if err != nil {
		w.Warning("git SSL config: Can't parse repository URL. %s", err)
		return
	}

	repoURL.Path = ""
	host := repoURL.String()
	variables := build.GetCITLSVariables()
	args := append([]string{"config"}, where...)

	for variable, config := range map[string]string{
		tls.VariableCAFile:   "sslCAInfo",
		tls.VariableCertFile: "sslCert",
		tls.VariableKeyFile:  "sslKey",
	} {
		if variables.Get(variable) == "" {
			continue
		}

		key := fmt.Sprintf("http.%s.%s", host, config)
		w.Command("git", append(args, key, w.EnvVariableKey(variable))...)
	}

	return
}

func (b *AbstractShell) writeCloneFetchCmds(w ShellWriter, info common.ShellScriptInfo) error {
	build := info.Build

	b.disableLFSSmudge(w, build)

	err := b.handleGetSourcesStrategy(w, build)
	if err != nil {
		return err
	}

	return b.handleCheckoutAndSubmodulesStrategy(w, build)
}

func (b *AbstractShell) disableLFSSmudge(w ShellWriter, build *common.Build) {
	// If LFS smudging was disabled by the user (by setting the GIT_LFS_SKIP_SMUDGE variable
	// when defining the job) we're skipping this step.
	//
	// In other case we're disabling smudging here to prevent us from memory
	// allocation failures.
	//
	// Please read https://gitlab.com/gitlab-org/gitlab-runner/issues/3366 and
	// https://github.com/git-lfs/git-lfs/issues/3524 for context.
	if build.IsLFSSmudgeDisabled() {
		return
	}

	w.Variable(common.JobVariable{Key: "GIT_LFS_SKIP_SMUDGE", Value: "1"})
}

func (b *AbstractShell) handleGetSourcesStrategy(w ShellWriter, build *common.Build) error {
	projectDir := build.FullProjectDir()
	gitDir := path.Join(build.FullProjectDir(), ".git")

	switch build.GetGitStrategy() {
	case common.GitFetch:
		b.writeRefspecFetchCmd(w, build, projectDir, gitDir)
	case common.GitClone:
		w.RmDir(projectDir)
		b.writeRefspecFetchCmd(w, build, projectDir, gitDir)
	case common.GitNone:
		w.Notice("Skipping Git repository setup")
		w.MkDir(projectDir)
	default:
		return errors.New("unknown GIT_STRATEGY")
	}

	return nil
}

func (b *AbstractShell) writeRefspecFetchCmd(w ShellWriter, build *common.Build, projectDir string, gitDir string) {
	depth := build.GitInfo.Depth

	if depth > 0 {
		w.Notice("Fetching changes with git depth set to %d...", depth)
	} else {
		w.Notice("Fetching changes...")
	}

	// initializing
	templateDir := w.MkTmpDir("git-template")
	templateFile := path.Join(templateDir, "config")

	w.Command("git", "config", "-f", templateFile, "fetch.recurseSubmodules", "false")
	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, []string{"-f", templateFile})
	}

	w.Command("git", "init", projectDir, "--template", templateDir)
	w.Cd(projectDir)
	b.writeGitCleanup(w, build)

	// Add `git remote` or update existing
	w.IfCmd("git", "remote", "add", "origin", build.GetRemoteURL())
	w.Notice("Created fresh repository.")
	w.Else()
	w.Command("git", "remote", "set-url", "origin", build.GetRemoteURL())
	w.EndIf()

	fetchArgs := []string{"fetch", "origin", "--prune"}
	fetchArgs = append(fetchArgs, build.GitInfo.Refspecs...)
	if depth > 0 {
		fetchArgs = append(fetchArgs, "--depth", strconv.Itoa(depth))
	}

	w.Command("git", fetchArgs...)
}

func (b *AbstractShell) writeGitCleanup(w ShellWriter, build *common.Build) {
	// Remove .git/{index,shallow,HEAD}.lock files from .git, which can fail the fetch command
	// The file can be left if previous build was terminated during git operation
	w.RmFile(".git/index.lock")
	w.RmFile(".git/shallow.lock")
	w.RmFile(".git/HEAD.lock")

	w.RmFile(".git/hooks/post-checkout")
}

func (b *AbstractShell) handleCheckoutAndSubmodulesStrategy(w ShellWriter, build *common.Build) error {
	if build.IsFeatureFlagOn(featureflags.UseLegacyGitCheckoutAndSubmodulesStrategy) {
		return b.handleOldCheckoutAndSubmodulesStrategy(w, build)
	}

	return b.handleNewCheckoutAndSubmodulesStrategy(w, build)
}

// TODO: Remove in XX.XX
//
// DEPRECATED
func (b *AbstractShell) handleOldCheckoutAndSubmodulesStrategy(w ShellWriter, build *common.Build) error {
	if build.GetGitCheckout() {
		b.writeCheckoutCmd(w, build)
		b.writeCleanCmd(w, build)
		b.writeLFSPullCmd(w, build)
	} else {
		w.Notice("Skipping Git checkout")
	}

	return b.handleSubmodulesStrategy(w, build)
}

// TODO: In XX.XX move it to 'handleCheckoutAndSubmodulesStrategy' and
//       make it the only supported strategy
//
// DEPRECATED
func (b *AbstractShell) handleNewCheckoutAndSubmodulesStrategy(w ShellWriter, build *common.Build) error {
	if !build.GetGitCheckout() {
		w.Notice("Skipping Git checkout")
		return nil
	}

	b.writeCheckoutCmd(w, build)
	b.writeCleanCmd(w, build)
	b.writeLFSPullCmd(w, build)

	return b.handleSubmodulesStrategy(w, build)
}

func (b *AbstractShell) writeCheckoutCmd(w ShellWriter, build *common.Build) {
	w.Notice("Checking out %s as %s...", build.GitInfo.Sha[0:8], build.GitInfo.Ref)
	w.Command("git", "checkout", "-f", "-q", build.GitInfo.Sha)
}

func (b *AbstractShell) writeCleanCmd(w ShellWriter, build *common.Build) {
	b.writeCleanCmdWthArgs(w, build, b.getCleanCmdArgs(build)...)
}

func (b *AbstractShell) writeCleanCmdWthArgs(w ShellWriter, build *common.Build, args ...string) {
	b.guardCleanCmd(w, build, args...)
}

func (b *AbstractShell) guardCleanCmd(w ShellWriter, build *common.Build, args ...string) {
	if len(build.GetGitCleanFlags()) <= 0 {
		return
	}

	w.Command("git", args...)
}

func (b *AbstractShell) getCleanCmdArgs(build *common.Build, args ...string) []string {
	cleanArgs := append(args, "clean")
	cleanArgs = append(cleanArgs, build.GetGitCleanFlags()...)

	return cleanArgs
}

func (b *AbstractShell) writeLFSPullCmd(w ShellWriter, build *common.Build) {
	b.writeLFSPullCmdWithArgs(w, build, b.getLFSPullCmdArgs(build)...)
}

func (b *AbstractShell) writeLFSPullCmdWithArgs(w ShellWriter, build *common.Build, args ...string) {
	b.guardLFSPullCmd(w, build, args...)
}

func (b *AbstractShell) guardLFSPullCmd(w ShellWriter, build *common.Build, args ...string) {
	// If LFS smudging was disabled by the user (by setting the GIT_LFS_SKIP_SMUDGE variable
	// when defining the job) we're skipping this step.
	//
	// In other case, because we've disabled LFS smudging above, we need now manually call
	// `git lfs pull` to fetch and checkout all LFS objects that may be present in
	// the repository.
	//
	// Repositories without LFS objects (and without any LFS metadata) will be not
	// affected by this command.
	//
	// Please read https://gitlab.com/gitlab-org/gitlab-runner/issues/3366 and
	// https://github.com/git-lfs/git-lfs/issues/3524 for context.
	if build.IsLFSSmudgeDisabled() {
		return
	}

	w.IfCmd("git-lfs", "version")
	w.Command("git", args...)
	w.EndIf()
}

func (b *AbstractShell) getLFSPullCmdArgs(build *common.Build, args ...string) []string {
	return append(args, "lfs", "pull")
}

func (b *AbstractShell) handleSubmodulesStrategy(w ShellWriter, build *common.Build) (err error) {
	switch build.GetSubmoduleStrategy() {
	case common.SubmoduleNormal:
		w.Notice("Updating/initializing submodules...")
		b.writeSubmoduleUpdateCmd(w, build, false)

	case common.SubmoduleRecursive:
		w.Notice("Updating/initializing submodules recursively...")
		b.writeSubmoduleUpdateCmd(w, build, true)

	case common.SubmoduleNone:
		w.Notice("Skipping Git submodules setup")

	default:
		return errors.New("unknown GIT_SUBMODULE_STRATEGY")
	}

	return nil
}

func (b *AbstractShell) writeSubmoduleUpdateCmd(w ShellWriter, build *common.Build, recursive bool) {
	// Sync .git/config to .gitmodules in case URL changes (e.g. new build token)
	args := withRecursive(recursive, "submodule", "sync")
	w.Command("git", args...)

	foreachArgs := withRecursive(recursive, "submodule", "foreach")

	// Clean changed files in submodules
	// "git submodule update --force" option not supported in Git 1.7.1 (shipped with CentOS 6)
	cleanArgs := append(foreachArgs, wrapArgsToString(b.getCleanCmdArgs(build, "git")...))
	b.writeCleanCmdWthArgs(w, build, cleanArgs...)

	resetArgs := append(foreachArgs, wrapArgsToString("git", "reset", "--hard"))
	w.Command("git", resetArgs...)

	// Update/initialize submodules
	updateArgs := withRecursive(recursive, "submodule", "update", "--init")
	w.Command("git", updateArgs...)

	// Update LFS objects within submodules
	lfsArgs := append(foreachArgs, wrapArgsToString(b.getLFSPullCmdArgs(build, "git")...))
	b.writeLFSPullCmdWithArgs(w, build, lfsArgs...)
}

func withRecursive(recursive bool, args ...string) []string {
	if !recursive {
		return args
	}

	return append(args, "--recursive")
}

func wrapArgsToString(args ...string) string {
	return strings.Join(args, " ")
}

func (b *AbstractShell) writeRestoreCacheScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Try to restore from main cache, if not found cache for master
	return b.cacheExtractor(w, info)
}

func (b *AbstractShell) writeDownloadArtifactsScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Process all artifacts
	b.downloadAllArtifacts(w, info)
	return nil
}

// Write the given string of commands using the provided ShellWriter object.
func (b *AbstractShell) writeCommands(w ShellWriter, commands ...string) {
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			lines := strings.SplitN(command, "\n", 2)
			if len(lines) > 1 {
				// TODO: this should be collapsable once we introduce that in GitLab
				w.Notice("$ %s # collapsed multi-line command", lines[0])
			} else {
				w.Notice("$ %s", lines[0])
			}
		} else {
			w.EmptyLine()
		}
		w.Line(command)
		w.CheckForErrors()
	}
}

func (b *AbstractShell) writeUserScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	var scriptStep *common.Step
	for _, step := range info.Build.Steps {
		if step.Name == common.StepNameScript {
			scriptStep = &step
			break
		}
	}

	if scriptStep == nil {
		return nil
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	if info.PreBuildScript != "" {
		b.writeCommands(w, info.PreBuildScript)
	}

	b.writeCommands(w, scriptStep.Script...)

	if info.PostBuildScript != "" {
		b.writeCommands(w, info.PostBuildScript)
	}

	return nil
}

func (b *AbstractShell) cacheArchiver(w ShellWriter, info common.ShellScriptInfo) error {
	for _, cacheOptions := range info.Build.Cache {
		// Skip archiving if no cache is defined
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Notice("Skipping cache archiving due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPush); err != nil {
			return fmt.Errorf("%s for %s", err, cacheKey)
		} else if !ok {
			w.Notice("Not uploading cache %s due to policy", cacheKey)
			continue
		}

		args := []string{
			"cache-archiver",
			"--file", cacheFile,
			"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
		}

		// Create list of files to archive
		archiverArgs := []string{}
		for _, path := range cacheOptions.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if cacheOptions.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		if len(archiverArgs) < 1 {
			// Skip creating archive
			continue
		}
		args = append(args, archiverArgs...)

		// Generate cache upload address
		if url := cache.GetCacheUploadURL(info.Build, cacheKey); url != nil {
			args = append(args, "--url", url.String())
		}

		// Execute cache-archiver command. Failure is not fatal.
		b.guardRunnerCommand(w, info.RunnerCommand, "Creating cache", func() {
			w.Notice("Creating cache %s...", cacheKey)
			w.IfCmdWithOutput(info.RunnerCommand, args...)
			w.Notice("Created cache")
			w.Else()
			w.Warning("Failed to create cache")
			w.EndIf()
		})
	}

	return nil
}

func (b *AbstractShell) writeUploadArtifact(w ShellWriter, info common.ShellScriptInfo, artifact common.Artifact) {
	args := []string{
		"artifacts-uploader",
		"--url",
		info.Build.Runner.URL,
		"--token",
		info.Build.Token,
		"--id",
		strconv.Itoa(info.Build.ID),
	}

	// Create list of files to archive
	archiverArgs := []string{}
	for _, path := range artifact.Paths {
		archiverArgs = append(archiverArgs, "--path", path)
	}

	if artifact.Untracked {
		archiverArgs = append(archiverArgs, "--untracked")
	}

	if len(archiverArgs) < 1 {
		// Skip creating archive
		return
	}
	args = append(args, archiverArgs...)

	if artifact.Name != "" {
		args = append(args, "--name", artifact.Name)
	}

	if artifact.ExpireIn != "" {
		args = append(args, "--expire-in", artifact.ExpireIn)
	}

	if artifact.Format != "" {
		args = append(args, "--artifact-format", string(artifact.Format))
	}

	if artifact.Type != "" {
		args = append(args, "--artifact-type", artifact.Type)
	}

	b.guardRunnerCommand(w, info.RunnerCommand, "Uploading artifacts", func() {
		w.Notice("Uploading artifacts...")
		w.Command(info.RunnerCommand, args...)
	})
}

func (b *AbstractShell) writeUploadArtifacts(w ShellWriter, info common.ShellScriptInfo, onSuccess bool) {
	if info.Build.Runner.URL == "" {
		return
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	for _, artifact := range info.Build.Artifacts {
		if onSuccess {
			if !artifact.When.OnSuccess() {
				continue
			}
		} else {
			if !artifact.When.OnFailure() {
				continue
			}
		}

		b.writeUploadArtifact(w, info, artifact)
	}
}

func (b *AbstractShell) writeAfterScript(w ShellWriter, info common.ShellScriptInfo) error {
	var afterScriptStep *common.Step
	for _, step := range info.Build.Steps {
		if step.Name == common.StepNameAfterScript {
			afterScriptStep = &step
			break
		}
	}

	if afterScriptStep == nil {
		return nil
	}

	if len(afterScriptStep.Script) == 0 {
		return nil
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	w.Notice("Running after script...")
	b.writeCommands(w, afterScriptStep.Script...)
	return nil
}

func (b *AbstractShell) writeArchiveCacheScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Find cached files and archive them
	return b.cacheArchiver(w, info)
}

func (b *AbstractShell) writeUploadArtifactsOnSuccessScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeUploadArtifacts(w, info, true)
	return
}

func (b *AbstractShell) writeUploadArtifactsOnFailureScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeUploadArtifacts(w, info, false)
	return
}

func (b *AbstractShell) writeScript(w ShellWriter, buildStage common.BuildStage, info common.ShellScriptInfo) error {
	methods := map[common.BuildStage]func(ShellWriter, common.ShellScriptInfo) error{
		common.BuildStagePrepare:                  b.writePrepareScript,
		common.BuildStageGetSources:               b.writeGetSourcesScript,
		common.BuildStageRestoreCache:             b.writeRestoreCacheScript,
		common.BuildStageDownloadArtifacts:        b.writeDownloadArtifactsScript,
		common.BuildStageUserScript:               b.writeUserScript,
		common.BuildStageAfterScript:              b.writeAfterScript,
		common.BuildStageArchiveCache:             b.writeArchiveCacheScript,
		common.BuildStageUploadOnSuccessArtifacts: b.writeUploadArtifactsOnSuccessScript,
		common.BuildStageUploadOnFailureArtifacts: b.writeUploadArtifactsOnFailureScript,
	}

	fn := methods[buildStage]
	if fn == nil {
		return errors.New("Not supported script type: " + string(buildStage))
	}

	return fn(w, info)
}
