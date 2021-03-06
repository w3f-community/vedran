FROM golang:alpine AS builder

# Set necessary environment variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -ldflags "-X 'github.com/NodeFactoryIo/vedran/pkg/version.Version=$(sed -n 's/version=//p' .version)'" -o vedran .

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/vedran .

# Build a small image
FROM scratch

COPY --from=builder /dist/vedran /

# Command to run
ENTRYPOINT ["/vedran"]
