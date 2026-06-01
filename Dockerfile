# Builder runs natively on the build host (BUILDPLATFORM) and cross-compiles to
# the requested target. CGO is disabled, so this is a pure Go cross-compile with
# no emulation — fast even when building arm64 on an amd64 runner.
FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS builder

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

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/plusplus /bin/plusplus

EXPOSE 8080

ENTRYPOINT ["/bin/plusplus"]
