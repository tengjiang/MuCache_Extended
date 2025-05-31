FROM golang:alpine

# Install build dependencies
RUN apk add --no-cache gcc musl-dev zeromq-dev iftop

# Set working directory
WORKDIR /app

# Copy Go mod files and download dependencies first
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd /app/cmd
COPY ./internal /app/internal
COPY ./pkg /app/pkg

# Set environment variables (can be overridden later)
ENV APP_NAME=main
ENV APP_NAMESPACE=frontend
ENV NODE_IDX=1
ENV CM_ENABLED=false
ENV EXPIRATION_TTL=0
ENV PREFETCH=${PREFETCH}

# Final runtime command: use if-else to decide
CMD if [ "$CM_ENABLED" = "true" ]; then \
        CGO_ENABLED=1 GOOS=linux go run -tags "k8s,node${NODE_IDX},cm" /app/cmd/${APP_NAMESPACE}/${APP_NAME}/main.go -prefetch=${PREFETCH} -logtostderr=true; \
    elif [ "$CM_ENABLED" = "upper" ]; then \
        CGO_ENABLED=1 GOOS=linux go run -tags "k8s,node${NODE_IDX},upperbound" /app/cmd/${APP_NAMESPACE}/${APP_NAME}/main.go -prefetch=${PREFETCH} -logtostderr=true; \
    else \
        CGO_ENABLED=1 GOOS=linux go run -tags "k8s,node${NODE_IDX}" /app/cmd/${APP_NAMESPACE}/${APP_NAME}/main.go -prefetch=${PREFETCH} -logtostderr=true; \
    fi
