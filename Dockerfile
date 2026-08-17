FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /modship ./cmd/modship

FROM alpine:latest
RUN apk add --no-cache docker-cli docker-cli-compose
COPY --from=builder /modship /usr/local/bin/modship

EXPOSE 8080
ENTRYPOINT ["modship"]
