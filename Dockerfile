# 1. Use the official Go image
FROM golang:1.25.3-alpine

# Install Docker CLI so the engine can run Docker commands
RUN apk add --no-cache docker-cli

# 2. Set the working directory inside the container
WORKDIR /app

# 3. Copy all your Go files into the container
COPY . .

# 4. Download any required Go modules (like Gin and Redis)
RUN go mod download

# 5. Compile the Go application
RUN go build -o engine .

# 6. Run the compiled binary when the container boots
CMD ["./engine"]