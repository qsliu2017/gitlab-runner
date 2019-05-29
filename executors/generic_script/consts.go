package generic_script

import "time"

const gracePeriodDeadline = 10 * time.Second
const killDeadline = 10 * time.Minute
const prepareScriptTimeout = time.Hour
const cleanupScriptTimeout = time.Hour
