package generic_script

import "time"

const defaultPrepareScriptTimeout = time.Hour
const defaultCleanupScriptTimeout = time.Hour

const defaultProcessKillTimeout = 10 * time.Minute
const defaultProcessKillGracePeriod = 10 * time.Second
