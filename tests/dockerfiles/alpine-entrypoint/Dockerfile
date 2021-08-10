FROM alpine:latest

run apk --no-cache add su-exec

COPY ./my-entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]
