FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/plusplus ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/plusplus /bin/plusplus

EXPOSE 8080

ENTRYPOINT ["/bin/plusplus"]
