// +build !windows

package generic_script

import (
	"errors"

	terminalsession "gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (e *executor) Connect() (terminalsession.Conn, error) {
	return nil, errors.New("not yet supported")
}
