# Switch to 1.25 so the libraries are happy
FROM golang:1.25-rc-alpine AS builder

WORKDIR /app

# Disable the toolchain check that causes the "RC" error
ENV GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Final Stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY virtual-items.json .

EXPOSE 8080
CMD ["./main"]