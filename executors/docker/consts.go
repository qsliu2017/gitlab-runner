package docker

import "time"

const prebuiltImageExtension = ".tar"

const dockerCleanupTimeout = 5 * time.Minute

const waitForContainerTimeout = 15 * time.Second

const osTypeLinux = "linux"
const osTypeWindows = "windows"

const metadataOSType = "OSType"
