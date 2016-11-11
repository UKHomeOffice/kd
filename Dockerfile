FROM alpine:3.4

RUN apk upgrade --no-cache
RUN apk add --no-cache ca-certificates openssl bash
ADD misc/kdaemon /usr/bin/kdaemon
RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.3.5/bin/linux/amd64/kubectl \
  -O /usr/bin/kubectl && chmod +x /usr/bin/kubectl

COPY bin/kd_linux_amd64 /bin/kd

ENTRYPOINT ["/bin/kd"]
