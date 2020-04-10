package secret_engines

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

func TestErrOperationNotSupported_Error(t *testing.T) {
	e := new(vault.MockSecretEngine)
	e.On("EngineName").
		Return("test-engine").
		Times(3)

	assert.Equal(t, `operation "get" for secret engine "test-engine" is not supported`, NewUnsupportedGetOperationErr(e).Error())
	assert.Equal(t, `operation "put" for secret engine "test-engine" is not supported`, NewUnsupportedPutOperationErr(e).Error())
	assert.Equal(t, `operation "delete" for secret engine "test-engine" is not supported`, NewUnsupportedDeleteOperationErr(e).Error())
}

func TestErrOperationNotSupported_Is(t *testing.T) {
	e := new(vault.MockSecretEngine)
	e.On("EngineName").
		Return("test-engine").
		Times(3)

	assert.True(t, errors.Is(NewUnsupportedGetOperationErr(e), new(ErrOperationNotSupported)))
	assert.True(t, errors.Is(NewUnsupportedPutOperationErr(e), new(ErrOperationNotSupported)))
	assert.True(t, errors.Is(NewUnsupportedDeleteOperationErr(e), new(ErrOperationNotSupported)))
}
