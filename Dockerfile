FROM golang:1.20 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/failover cmd/main.go

FROM scratch

WORKDIR /app

COPY --from=builder /app/failover /app/failover

ENTRYPOINT [ "/app/failover" ]
