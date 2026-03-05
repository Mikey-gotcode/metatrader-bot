# 1. Start with the Go compiler image
FROM golang:1.21-alpine AS builder

# 2. Set the working directory
WORKDIR /app

# 3. Copy module files first (to cache dependencies)
COPY go.mod go.sum ./

# 4. Download the modules
RUN go mod download && go mod verify

# 5. Copy the source code
COPY . .

# 6. Build the application
RUN go build -o /trade_service main.go

# 7. Use a tiny image for the final run (Production style)
FROM alpine:latest
WORKDIR /
COPY --from=builder /trade_service /trade_service

# Expose the port your Python bot hits
EXPOSE 8080

CMD ["./trade_service"]
