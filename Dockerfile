# Builder runs natively on the build host (BUILDPLATFORM) and cross-compiles to
# the requested target. CGO is disabled, so this is a pure Go cross-compile with
# no emulation — fast even when building arm64 on an amd64 runner.
FROM --platform=$BUILDPLATFORM golang:1.26.11-alpine@sha256:ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY package.json ./

# Provided automatically by buildx for each --platform.
ARG TARGETOS
ARG TARGETARCH

RUN VERSION="$(sed -n 's/.*"version" *: *"\([^"]*\)".*/\1/p' package.json | head -n1)" \
    && [ -n "$VERSION" ] \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
       go build -ldflags "-X plusplus/internal/version.Version=${VERSION}" -o /bin/plusplus ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=builder /bin/plusplus /bin/plusplus

EXPOSE 8080

ENTRYPOINT ["/bin/plusplus"]
