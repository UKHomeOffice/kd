FROM alpine:3.4

RUN apk upgrade --no-cache
RUN apk add --no-cache ca-certificates

COPY bin/kd_linux_amd64 /bin/kd

ENTRYPOINT ["/bin/kd"]
