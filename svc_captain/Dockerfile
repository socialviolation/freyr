FROM golang:1.21 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o captain .


FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/captain .
EXPOSE 5001
CMD ["./captain"]