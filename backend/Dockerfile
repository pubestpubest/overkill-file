FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o backend

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/backend ./backend
COPY --from=build /app/migrations ./migrations
EXPOSE 8080
CMD ["./backend"]
