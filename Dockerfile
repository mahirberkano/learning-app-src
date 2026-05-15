FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go .
COPY static/ ./static/
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server .

FROM scratch
COPY --from=builder /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
