FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add --no-cache git nodejs npm

WORKDIR /app

# Install frontend dependencies
COPY frontend/package*.json frontend/
RUN cd frontend && npm ci

# Copy all sources
COPY . .

# Build frontend (outputs to static/dist/)
RUN cd frontend && npm run build

# Build Go binary (go:embed picks up static/dist/ and config/)
ARG TARGETARCH TARGETOS
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o live-actions

# Final stage — single binary
FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/live-actions /usr/local/bin/

RUN mkdir -p /app/data

CMD ["live-actions"]