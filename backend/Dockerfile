FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o backend

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/backend ./backend
EXPOSE 8080
CMD ["./backend"]
