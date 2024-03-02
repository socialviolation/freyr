FROM golang:1.22 as builder
WORKDIR /app/modes
COPY modes .
WORKDIR /app/svc_captain
COPY svc_captain/go.mod svc_captain/go.sum svc_captain/go.work svc_captain/go.work.sum ./
RUN go work use
RUN go mod download
RUN go mod verify
COPY svc_captain .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /captain main.go


FROM gcr.io/distroless/static-debian11
COPY --from=builder /captain .
EXPOSE 5001
CMD ["/captain"]
