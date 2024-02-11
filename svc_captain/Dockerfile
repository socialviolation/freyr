FROM golang:1.21 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /captain .


FROM gcr.io/distroless/static-debian11
COPY --from=builder /captain .
EXPOSE 5001
CMD ["/captain"]
