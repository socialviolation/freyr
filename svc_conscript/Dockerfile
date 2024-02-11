FROM golang:1.21 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o conscript .


FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/conscript .
EXPOSE 5003
CMD ["./conscript"]