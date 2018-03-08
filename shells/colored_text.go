package shells

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func coloredText(color, format string, arguments ...interface{}) string {
	return color + fmt.Sprintf(format, arguments...) + formatter.ANSI_RESET
}

func normalText(format string, arguments ...interface{}) string {
	return coloredText(formatter.ANSI_RESET, format, arguments...)
}

func noticeText(format string, arguments ...interface{}) string {
	return coloredText(formatter.ANSI_BOLD_GREEN, format, arguments...)
}

func warningText(format string, arguments ...interface{}) string {
	return coloredText(formatter.ANSI_YELLOW, format, arguments...)
}

func errorText(format string, arguments ...interface{}) string {
	return coloredText(formatter.ANSI_BOLD_RED, format, arguments...)
}
