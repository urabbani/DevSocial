# Stage 1: Build React frontend
FROM node:22-alpine AS frontend

WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.26-alpine AS backend

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o devsocial ./cmd/devsocial

# Stage 3: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend /app/devsocial .
COPY --from=backend /app/web/dist ./web/dist
COPY --from=backend /app/internal/database/migrations ./internal/database/migrations
COPY --from=backend /app/static ./static

RUN mkdir -p /data/uploads

EXPOSE 8080

CMD ["./devsocial", "-addr", ":8080"]
