package common

type BuildStage struct {
	Name          string
	Description   string
	PredefinedEnv bool
	AttemptsKey   string
	Step          Step
}

var (
	BuildStageResolveSecrets           = BuildStage{Name: "resolve_secrets", PredefinedEnv: true, Description: "Resolving secrets"}
	BuildStagePrepareExecutor          = BuildStage{Name: "prepare_executor", PredefinedEnv: true, Description: "Preparing environment"}
	BuildStagePrepare                  = BuildStage{Name: "prepare_script", PredefinedEnv: true, Description: "Preparing scripts"}
	BuildStageGetSources               = BuildStage{Name: "get_sources", PredefinedEnv: true, AttemptsKey: "GET_SOURCES_ATTEMPTS", Description: "Getting source from Git repository"}
	BuildStageRestoreCache             = BuildStage{Name: "restore_cache", PredefinedEnv: true, AttemptsKey: "RESTORE_CACHE_ATTEMPTS", Description: "Restoring cache"}
	BuildStageUserScript               = BuildStage{Name: "user_step", PredefinedEnv: false, Description: "Executing user step"}
	BuildStageAfterScript              = BuildStage{Name: "after_script", PredefinedEnv: false, Description: "Running after_script"}
	BuildStageDownloadArtifacts        = BuildStage{Name: "download_artifacts", PredefinedEnv: true, AttemptsKey: "ARTIFACT_DOWNLOAD_ATTEMPTS", Description: "Downloading artifacts"}
	BuildStageArchiveCache             = BuildStage{Name: "archive_cache", PredefinedEnv: true, Description: "Preparing environment"}
	BuildStageUploadOnSuccessArtifacts = BuildStage{Name: "upload_artifacts_on_success", PredefinedEnv: true, Description: "Uploading artifacts for successful job"}
	BuildStageUploadOnFailureArtifacts = BuildStage{Name: "upload_artifacts_on_failure", PredefinedEnv: true, Description: "Uploading artifacts for failed job"}
)

// staticBuildStages is a list of BuildStages which are executed on every build
// and are not dynamically generated from steps.
var prefixedBuildStages = []BuildStage{
	BuildStagePrepare,
	BuildStageGetSources,
	BuildStageRestoreCache,
	BuildStageDownloadArtifacts,
}

var postfixBuildStages = []BuildStage{
	BuildStageArchiveCache,
	BuildStageUploadOnSuccessArtifacts,
	BuildStageUploadOnFailureArtifacts,
}
