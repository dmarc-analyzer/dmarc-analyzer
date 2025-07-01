# Stage 1: Build the frontend
FROM node:22 AS frontend-builder

# Set the working directory inside the container
WORKDIR /app

# Copy package.json and package-lock.json files
COPY frontend/package.json frontend/package-lock.json ./

# Install dependencies
RUN npm install

# Copy the frontend source code
COPY frontend/ ./

# Build the frontend application
RUN npm run build

# Stage 2: Build the backend
FROM golang:1.24-alpine AS backend-builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to the workspace
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY backend/ ./backend/

# Build the main server application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./backend/cmd/server/server.go

# Build the backfill command line tool
RUN CGO_ENABLED=0 GOOS=linux go build -o backfill ./backend/cmd/backfill/backfill.go

# Build the SQS consumer
RUN CGO_ENABLED=0 GOOS=linux go build -o consumer ./backend/cmd/consumer/consumer.go

# Stage 3: Final image
FROM alpine:latest

# Install CA certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /app/

# Copy the binaries from the backend builder stage
COPY --from=backend-builder /app/server .
COPY --from=backend-builder /app/backfill .
COPY --from=backend-builder /app/consumer .

# Create directory for static files
RUN mkdir -p /app/frontend/dist/

# Copy the frontend build from the frontend builder stage
COPY --from=frontend-builder /app/dist/ /app/frontend/dist/

# Expose the port the server runs on
EXPOSE 6767

# Command to run when the container starts (default to server)
CMD ["./server"]
