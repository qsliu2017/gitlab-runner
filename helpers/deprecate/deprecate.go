package deprecate

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const EnvPrefix = "SUPPRESS_DEPRECATION_"

var suppressions map[string]struct{}

func init() {
	suppressions = make(map[string]struct{})

	suppressEnv(os.Environ())
}

// DeprecationLogger is an interface representing a simple logger that supports
// Warningln and Debugln.
type DeprecationLogger interface {
	Warningln(args ...interface{})
	Debugln(args ...interface{})
}

func suppressEnv(environ []string) {
	for _, env := range environ {
		if !strings.HasPrefix(env, EnvPrefix) {
			continue
		}

		kv := strings.SplitN(env, "=", 2)
		enabled := len(kv) == 1
		if len(kv) > 1 {
			enabled, _ = strconv.ParseBool(kv[1])
		}

		if !enabled {
			continue
		}

		issue := strings.TrimPrefix(kv[0], EnvPrefix)
		suppressions[issue] = struct{}{}
	}
}

// Warningln logs a deprecation warning:
//
// Warningln(logger, "2227", "14.0 will replace the 'build_script' with 'step_script'")
//   will output:
// deprecation[2227](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2227):
//   14.0 will replace the 'build_script' with 'step_script'
func Warningln(logger DeprecationLogger, issue string, args ...interface{}) {
	if _, ok := suppressions[issue]; ok {
		return
	}

	logger.Warningln(logArgs(issue, args)...)
}

func Debugln(logger DeprecationLogger, issue string, args ...interface{}) {
	if _, ok := suppressions[issue]; ok {
		return
	}

	logger.Debugln(logArgs(issue, args)...)
}

func logArgs(issue string, args []interface{}) []interface{} {
	prefix := fmt.Sprintf("deprecation[%s](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/%s):", issue, issue)

	return append([]interface{}{prefix}, args...)
}
