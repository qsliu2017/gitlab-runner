package secret_engines

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

type OperationType string

const (
	getOperation    OperationType = "get"
	putOperation    OperationType = "put"
	deleteOperation OperationType = "delete"
)

type ErrOperationNotSupported struct {
	secretEngineName string
	operationType    OperationType
}

func NewUnsupportedGetOperationErr(engine vault.SecretEngine) *ErrOperationNotSupported {
	return newErrOperationNotSupported(engine, getOperation)
}

func NewUnsupportedPutOperationErr(engine vault.SecretEngine) *ErrOperationNotSupported {
	return newErrOperationNotSupported(engine, putOperation)
}

func NewUnsupportedDeleteOperationErr(engine vault.SecretEngine) *ErrOperationNotSupported {
	return newErrOperationNotSupported(engine, deleteOperation)
}

func newErrOperationNotSupported(engine vault.SecretEngine, operationType OperationType) *ErrOperationNotSupported {
	return &ErrOperationNotSupported{
		secretEngineName: engine.EngineName(),
		operationType:    operationType,
	}
}

func (e *ErrOperationNotSupported) Error() string {
	return fmt.Sprintf("operation %q for secret engine %q is not supported", e.operationType, e.secretEngineName)
}

func (e *ErrOperationNotSupported) Is(err error) bool {
	_, ok := err.(*ErrOperationNotSupported)

	return ok
}
