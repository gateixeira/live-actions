FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Setup cross-compilation
ARG TARGETARCH TARGETOS

# Disable CGO and build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o live-actions

# Final stage
FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/live-actions /usr/local/bin/
COPY static/ /app/static/
COPY config/ /app/config/

RUN mkdir -p /app/data

CMD ["live-actions"]