# Stage 1: Build the frontend
FROM node:22-alpine AS frontend-builder

# Set the working directory inside the container
WORKDIR /app

# Copy package.json and yarn.lock files
COPY frontend/package.json frontend/yarn.lock ./

# Install dependencies
RUN yarn install

# Copy the frontend source code
COPY frontend/ ./

# Build the frontend application
RUN yarn build --configuration=production

# Stage 2: Build the backend
FROM golang:1.24-alpine AS backend-builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to the workspace
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build ./backend/cmd/server/server.go

# Build the backfill command line tool
RUN CGO_ENABLED=0 GOOS=linux go build ./backend/cmd/backfill/backfill.go

# Stage 3: Final image
FROM alpine:latest

# Install CA certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Create directory for static files
RUN mkdir -p /root/static

# Copy the binary from the backend builder stage
COPY --from=backend-builder /app/server .
COPY --from=backend-builder /app/backfill .

# Copy the frontend build from the frontend builder stage
COPY --from=frontend-builder /app/dist/ /root/static/

# Expose the port the server runs on
EXPOSE 6767

# Command to run when the container starts
CMD ["./server"]
