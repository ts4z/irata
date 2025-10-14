FROM golang:1.25 AS builder

ENV LISTEN=:8080
ENV DATABASE_URL=

EXPOSE 8080

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build .

FROM scratch

WORKDIR /app

COPY --from=builder /app/irata /irata

CMD ["/irata"]
