package response

import (
	"github.com/sirupsen/logrus"
)

const (
	defaultHandlerCode = -9999

	defaultLogLevel        = logrus.InfoLevel
	defaultFailureLogLevel = logrus.ErrorLevel

	CorrelationIDHeader = "X-Request-Id"
)

type Logger interface {
	logrus.FieldLogger

	Logln(level logrus.Level, args ...interface{})
}

type Handler struct {
	logger Logger
	resp   *Response

	logLine string

	handlers       map[int]handlerDefinition
	defaultHandler handlerDefinition

	handlerBuilders []*HandlerDefinitionBuilder
}

func NewHandler(logger Logger, logLine string) *Handler {
	return &Handler{
		logger:   logger,
		logLine:  logLine,
		handlers: make(map[int]handlerDefinition),
	}
}

// SetResponse sets the Response object that will be next
// handled by the handler and used to generate log
// fields
func (h *Handler) SetResponse(resp *Response) {
	h.resp = resp
}

// Log is intended to be used in error cases, when passing the response to `Handle()`
// doesn't make sense or even before we have the response available at all,
// but when we want to have a consistent logging configuration
func (h *Handler) Log(level logrus.Level, args ...interface{}) {
	h.logMessage(h.getLogger(), level, args...)
}

func (h *Handler) getLogger() Logger {
	if h.resp == nil {
		return h.logger
	}

	log := h.logger
	log = log.WithFields(logrus.Fields{
		"status":      h.resp.StatusCode(),
		"status_text": h.resp.Status(),
	})

	correlationID := h.resp.Header().Get(CorrelationIDHeader)
	if correlationID != "" {
		log = log.WithField("correlation_id", correlationID)
	}

	log = addAPIErrorToLogger(log, h.resp)

	return log
}

func (h *Handler) logMessage(logger Logger, logLevel logrus.Level, args ...interface{}) {
	defaultArgs := make([]interface{}, 0)
	defaultArgs = append(defaultArgs, h.logLine)
	defaultArgs = append(defaultArgs, args...)

	logger.Logln(logLevel, defaultArgs...)
}

// InDefaultCase will prepare a handler definition builder that will
// prepare the definition used for all responses for which we didn't specify
// a custom rule with the usage of `WhenCodeIs()`.
func (h *Handler) InDefaultCase() *HandlerDefinitionBuilder {
	return h.newHandlerDefinitionBuilder().asDefault()
}

func (h *Handler) newHandlerDefinitionBuilder() *HandlerDefinitionBuilder {
	b := newHandlerDefinitionBuilder()
	h.handlerBuilders = append(h.handlerBuilders, b)

	return b
}

// WhenCodeIs will prepare a handler definition builder that will prepare the
// definition used only for the specified response code.
func (h *Handler) WhenCodeIs(statusCode int) *HandlerDefinitionBuilder {
	return h.newHandlerDefinitionBuilder().withCode(statusCode)
}

// Flush is a cleanup method. It must be called as `defer h.Flush()`
// always when the Handler is created.
func (h *Handler) Flush() {
	if h.resp == nil {
		return
	}

	h.resp.discardBody()
}

// Handle is the main purpose of having the Handler struct at all. It will
// use the specified logging configuration, adding details taken from the
// response itself. In case of an error, it will properly read and log the error
// message sent by GitLab's API.
// Basing on the response code and the previously prepared definitions, it will
// select which definition to use. For matching code a specific definition will
// be used. In other case the definition registered as 'default' will be used.
// The definition may may update the log level (based on the response status or
// set custom by the user), add additional log fields. It can also execute a
// specified handler function, which is a way to hook with specific operations
// dependent on the received response.
func (h *Handler) Handle() interface{} {
	h.buildHandlers()

	log := h.getLogger()
	statusCode := h.getStatusCode()
	handler := h.getHandler(statusCode)

	h.logMessage(
		log.WithFields(handler.logFields),
		handler.logLevel,
		handler.logArgument,
	)

	if handler.handlerFn == nil {
		return nil
	}

	return handler.handlerFn(log)
}

func (h *Handler) buildHandlers() {
	h.defaultHandler = handlerDefinition{
		logLevel:    defaultFailureLogLevel,
		logArgument: "error",
	}
	h.handlers = make(map[int]handlerDefinition)

	for _, b := range h.handlerBuilders {
		h.register(b.handlerDefinition)
	}
}

func (h *Handler) register(definition handlerDefinition) {
	if definition.code == defaultHandlerCode {
		h.defaultHandler = definition
		return
	}

	h.handlers[definition.code] = definition
}

func (h *Handler) getStatusCode() int {
	if h.resp == nil {
		return 0
	}
	return h.resp.StatusCode()
}

func (h *Handler) getHandler(statusCode int) handlerDefinition {
	handler, ok := h.handlers[statusCode]
	if !ok {
		handler = h.defaultHandler
	}

	return handler
}

// AddLogFields is a facade to add arbitrary log fields to the
// internal logger object.
func (h *Handler) AddLogFields(fields logrus.Fields) *Handler {
	h.logger = h.logger.WithFields(fields)

	return h
}

// AddLogError is a facade to set the error field on the
// internal logger object.
func (h *Handler) AddLogError(err error) *Handler {
	h.logger = h.logger.WithError(err)

	return h
}
