# ---------------------------------------------------------------------------
# This script depends on a few environment variables that hsould be populated
# before running the script:
#
# - $Env:WINDOWS_VERSION - This is the version of windows that is going to be
#   used for building the Docker image. It is important for the version to match
#   one of the Dockerfile suffix, for example `nanoserver1809` for the Dockerfile
#   `Dockerfile.x86_64_nanoserver1809`
# - $Env:GIT_VERSON - Specify which version of Git needs to be isntalled on
#   the Docker image. This is done through Docker build args.
# - $Env:GIT_VERSON_BUILD - Specify which build is needed to download for the
#   GIT_VERSION you specified.
# - $Env:GIT_LFS_VERSION - The Git LFS version needed to installed on the
#   Docker image.
# - $Env:IS_LATEST - When we want to tag current tag as latsest, this is usally
#   used when we are tagging a release for the runner (which is not a patch
#   release or RC)
# - $Env:DOCKER_HUB_USER - The user we want to login with for docker hub.
# - $Env:DOCKER_HUB_PASSWORD - The password we want to login with for docker hub.
# ---------------------------------------------------------------------------
$dirSeperator = [IO.Path]::DirectorySeparatorChar # Get directory seperator depending on the OS.
$imagesBasePath = "dockerfiles$($dirSeperator)build$($dirSeperator)Dockerfile.x86_64"

function Main
{
    if (-not (Test-Path env:WINDOWS_VERSION))
    {
        throw '$Env:WINDOWS_VERSION is not set'
    }

    $tag = Get-Tag

    Build-Image $tag

    if (-not ($Env:PUSH_TO_DOCKER_HUB -eq "true"))
    {
        '$Env:PUSH_TO_DOCKER_HUB is not true, skipping'
        return
    }

    Connect-Registry

    Push-Tag $tag

    Disconnect-Registry
}

function Get-Tag
{
    $revision = & 'git' rev-parse --short=8 HEAD

    return "x86_64-$revision-$Env:WINDOWS_VERSION"
}

function Build-Image($tag)
{
    Write-Output "Building image for x86_64_$Env:WINDOWS_VERSION"

    $dockerFile = "${imagesBasePath}_$Env:WINDOWS_VERSION"
    $context = "dockerfiles$($dirSeperator)build"
    $buildArgs = @(
        '--build-arg', "GIT_VERSION=$Env:GIT_VERSION",
        '--build-arg', "GIT_VERSION_BUILD=$Env:GIT_VERSION_BUILD",
        '--build-arg', "GIT_LFS_VERSION=$Env:GIT_LFS_VERSION"
    )

    & 'docker' build -t "steveazz/gitlab-runner-helper:$tag" $buildArgs -f $dockerFile $context
}

function Push-Tag($tag)
{
    Write-Output "Pushing $tag"

    & 'docker' push steveazz/gitlab-runner-helper:$tag
}


function Connect-Registry
{
    Write-Output 'Loging in docker hub'

    & 'docker' login --username $Env:DOCKER_HUB_USER --password $Env:DOCKER_HUB_PASSWORD
}

function Disconnect-Registry
{
    Write-Output 'Loging out register'

    & 'docker' logout
}

Main
