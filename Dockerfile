FROM golang:1.17 as build-env

WORKDIR /go/src/cloudflare-scim
ADD . /go/src/cloudflare-scim

RUN --mount=type=cache,target=/go/pkg --mount=type=cache,target=/root/.cache/go-build go get -d -v ./...

RUN --mount=type=cache,target=/go/pkg --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags="-s -w" -o /go/bin/cloudflare-scim cmd/cloudflare-scim.go

FROM gcr.io/distroless/static-debian11

COPY --from=build-env /go/bin/cloudflare-scim /
CMD ["/cloudflare-scim"]
