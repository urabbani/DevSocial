FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o devsocial ./cmd/devsocial

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/devsocial .
COPY schema.sql .
COPY templates/ templates/
COPY static/ static/

EXPOSE 8888

CMD ["./devsocial", "-addr", ":8888", "-db", "/data/devsocial.db"]
