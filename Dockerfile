FROM golang:1.22.4-bookworm AS builder

WORKDIR /source
COPY . . 


ARG CGO_ENABLED=1
ARG GOPROXY=https://proxy.golang.org,direct 

RUN go build                                                        \
 && mkdir -p /build/usr/bin                                         \
 && cp go-tts /build/usr/bin/                                       \
 && cp -r ./vits-coqui-en-vctk/ /build/                             \
 && ldd /build/usr/bin/go-tts | tr -s '[:blank:]' '\n'              \
  | grep '^/'                                                       \
  | xargs -I % sh -c 'mkdir -p /build/$(dirname %); cp % /build/%;'

FROM scratch

COPY --from=builder --chown=1000:1000 /build /

USER 1000:1000

ENTRYPOINT [ "/usr/bin/go-tts" ]
