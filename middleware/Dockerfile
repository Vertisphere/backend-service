FROM golang:1.22.2-bullseye AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api cmd/api/main.go

# switch to smaller for prod 
FROM alpine:latest AS release

WORKDIR /app
COPY --from=build /app/api /app/api
EXPOSE 8080

CMD ["./api"]