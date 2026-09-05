# Build Stage
FROM golang:alpine AS builder

# Install ca-certificates and setup non-root user
RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o argo-c .

# Final Stage
FROM scratch

# Copy the ca-certificates from the builder stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the user and group files from the builder stage
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/argo-c /argo-c

# Use an unprivileged user
USER appuser:appuser

# Run the executable
ENTRYPOINT ["/argo-c"]
