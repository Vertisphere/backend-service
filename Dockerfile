# Use the official Golang image as the builder
FROM golang:1.22.2-bullseye AS build

# Set the working directory
WORKDIR /app

# Commented out since vendor
# Copy the Go modules files and download the dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -o backend cmd/backend/main.go

# Use a lightweight image for the final container
FROM gcr.io/distroless/base-debian11 AS release
WORKDIR /app

# WORKDIR /app
# Copy the compiled Go binary from the builder
COPY --from=build /app/backend /app/backend

# Expose the port on which the app will run
EXPOSE 8080

# Command to run the application
CMD ["./backend"]
