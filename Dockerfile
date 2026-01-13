FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o datasaver ./cmd/datasaver

FROM alpine:3.21

RUN apk add --no-cache \
    postgresql16-client \
    ca-certificates \
    tzdata

COPY --from=builder /app/datasaver /usr/local/bin/datasaver

RUN adduser -D -u 1000 datasaver
USER datasaver

EXPOSE 8080 9090

ENTRYPOINT ["datasaver"]
CMD ["daemon"]
