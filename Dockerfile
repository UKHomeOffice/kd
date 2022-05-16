FROM alpine:latest

RUN apk upgrade --no-cache
RUN apk add --no-cache ca-certificates openssl bash
RUN update-ca-certificates

RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.20.15/bin/linux/amd64/kubectl \
  -O /usr/bin/kubectl && chmod +x /usr/bin/kubectl

COPY bin/kd_linux_amd64 /bin/kd

RUN chmod +x /bin/kd

ENTRYPOINT ["/bin/kd"]
