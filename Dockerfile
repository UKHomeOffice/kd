FROM alpine:3.6

RUN apk upgrade --no-cache
RUN apk add --no-cache ca-certificates openssl bash

RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.8.4/bin/linux/amd64/kubectl \
  -O /usr/bin/kubectl && chmod +x /usr/bin/kubectl

COPY bin/kd_linux_amd64 /bin/kd

RUN chmod +x /bin/kd

RUN set -x ; \
    addgroup -S kd ; \
    adduser -D -S -G kd kd && exit 0 ; exit 1

USER kd

ENTRYPOINT ["/bin/kd"]
