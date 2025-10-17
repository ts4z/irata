FROM golang:1.25 AS builder

ENV LISTEN=:8080
ENV DATABASE_URL=
ENV DB_USER=
ENV DB_PASS=
ENV DB_NAME=
ENV INSTANCE_CONNECTION_NAME=
ENV PRIVATE_IP=

EXPOSE 8080

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build cmd/irata/irata.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/irata /irata

CMD ["/irata"]
