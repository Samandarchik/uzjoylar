# ------------------------------
# 1) Build stage
# ------------------------------
FROM golang:1.23 AS builder

WORKDIR /app

# Module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Go server
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go


# ------------------------------
# 2) Final stage
# ------------------------------
FROM alpine:3.19

WORKDIR /app

# Install required packages
RUN apk add --no-cache ca-certificates curl tzdata && update-ca-certificates

# Copy build output
COPY --from=builder /app/server /app/server

# Copy JSON data
COPY data /app/data

# Copy uploads (images)
COPY uploads /app/uploads

# Create directories if not exist
RUN mkdir -p /app/uploads && mkdir -p /app/data

# Expose port
EXPOSE 2020

# Run app
CMD ["./server"]
