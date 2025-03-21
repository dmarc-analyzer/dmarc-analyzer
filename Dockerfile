# Use the official Go image as a parent image
FROM golang:1.18-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to the workspace
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o dmarc-server ./backend/cmd/server/server.go

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install CA certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/dmarc-server .

# Expose the port the server runs on
EXPOSE 6767

# Command to run when the container starts
CMD ["./dmarc-server"]