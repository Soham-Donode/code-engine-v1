# --- STAGE 1: Build Stage ---
FROM golang:1.25.3-alpine AS builder

WORKDIR /app

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build a static binary (CGO_ENABLED=0 removes dynamic library dependencies)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o engine .


# --- STAGE 2: Minimal Runtime Stage ---
FROM alpine:latest

# Install Docker CLI so your app can communicate with docker.sock
RUN apk add --no-cache docker-cli ca-certificates

WORKDIR /app

# Copy ONLY the compiled binary from the builder stage
COPY --from=builder /app/engine .

# Run the compiled binary
CMD ["./engine"]