#
# Step 1
#

# Specify the version of Go to use
FROM golang:1.16 AS builder

WORKDIR /go/src/app

# Install upx (upx.github.io) to compress the compiled binary
RUN apt update && apt -y install upx

# Turn on Go modules support and disable CGO
# ENV GO111MODULE=on CGO_ENABLED=0

# Copy all the files from the host into the container
COPY . .

# Compile the Go code - the added flags instruct Go to produce a
# standalone binary
RUN go get -d -v ./...
RUN go build \
  -a \
  -trimpath \
  -ldflags "-s -w -extldflags '-static'" \
  # -installsuffix cgo \
  # -tags netgo \
  -o /bin/notarize-and-verify-commit \
  ./main.go

# Strip any symbols - this is not a library
RUN strip /bin/notarize-and-verify-commit

# Compress the compiled binary
RUN upx -q -9 /bin/notarize-and-verify-commit


# Step 2

# Use the most basic and empty container - this container has no
# runtime, files, shell, libraries, etc.
FROM scratch
# For testing, a more complete environment might be needed:
# FROM alpine:latest
# RUN apk update && apk upgrade && apk add --no-cache bash git

# Copy over SSL certificates from the first step - this is required
# if our code makes any outbound SSL connections because it contains
# the root CA bundle.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy over the compiled binary from the first step
COPY --from=builder /bin/notarize-and-verify-commit /bin/notarize-and-verify-commit

# Specify the container's entrypoint
ENTRYPOINT ["/bin/notarize-and-verify-commit"]