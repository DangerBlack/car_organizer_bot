FROM golang:1.24.1-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY src/ src/

RUN go build -o car_organizer_bot ./src/main.go
RUN chmod +x car_organizer_bot

FROM alpine:latest

RUN apk add --no-cache sqlite-libs curl tzdata

COPY --from=builder /app/car_organizer_bot .

ENV GIN_MODE=release

EXPOSE 3000

CMD ["./car_organizer_bot"]
