FROM golang:1.22 as builder
ENV GOPROXY=https://platform-gomodproxy.services.x.gcp.anz,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /conscript .


FROM gcr.io/distroless/static-debian11
COPY --from=builder /conscript .
EXPOSE 5003
CMD ["/conscript"]
