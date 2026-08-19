FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build -o /supervisor ./cmd/supervisor/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /supervisor /usr/local/bin/supervisor
EXPOSE 9000
ENTRYPOINT ["supervisor"]
