FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /fault-cli ./cmd/fault-cli

FROM alpine:3.19
RUN apk add --no-cache iproute2 ca-certificates
COPY --from=builder /fault-cli /usr/local/bin/fault-cli

ENTRYPOINT ["fault-cli"]
