FROM alpine:3.9

WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "/home/flux/flux-recv" ]

RUN apk add --no-cache ca-certificates tini

COPY ./flux-recv ./
