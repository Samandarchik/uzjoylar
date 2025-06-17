# Build stage
FROM golang:1.19-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy uploads directory if exists
COPY --from=builder /app/uploads ./uploads

# Create uploads directory if not exists
RUN mkdir -p uploads

# Expose port
EXPOSE 8000

# Set environment variables
ENV GIN_MODE=release
ENV PORT=8000

# Run the application
CMD ["./main"]