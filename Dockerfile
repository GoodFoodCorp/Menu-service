FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o menu-service ./cmd/main.go

FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder /app/menu-service /menu-service
USER nonroot:nonroot
EXPOSE 8085
ENTRYPOINT ["/menu-service"]
