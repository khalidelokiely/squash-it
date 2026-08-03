FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/squash-it main.go

RUN mkdir -p /app/data && chown -R 65532:65532 /app/data

FROM gcr.io/distroless/static-debian12:latest

WORKDIR /app

COPY --from=builder /app/squash-it /app/squash-it

COPY --from=builder --chown=nonroot:nonroot /app/data /app/data

USER nonroot:nonroot

ENTRYPOINT ["./squash-it"]