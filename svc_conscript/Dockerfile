FROM golang:1.24 AS builder
WORKDIR /app/shared
COPY shared .
WORKDIR /app/svc_conscript
COPY svc_conscript/go.mod svc_conscript/go.sum svc_conscript/go.work svc_conscript/go.work.sum ./
RUN go work use
RUN go mod download
RUN go mod verify
COPY svc_conscript .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /conscript main.go


FROM gcr.io/distroless/static-debian11
COPY --from=builder /conscript .
EXPOSE 5003
CMD ["/conscript"]
