FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o quizzo .

FROM alpine:3.19 AS production

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/quizzo /app/quizzo

EXPOSE 8080

ENTRYPOINT ["/app/quizzo"]
