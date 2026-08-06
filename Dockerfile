# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache dependencies first (go.mod/go.sum rarely change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inventra-api ./cmd/server

FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/inventra-api /app/inventra-api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/inventra-api"]