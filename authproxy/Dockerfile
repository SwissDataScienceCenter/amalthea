# Build the application from source
FROM golang:1.22 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux go build -o /authproxy

# Run the tests in the container
FROM build-stage AS run-test-stage
RUN go test -v ./...

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian12 AS build-release-stage

WORKDIR /

COPY --from=build-stage /authproxy /authproxy

USER nonroot:nonroot

ENTRYPOINT ["/authproxy"]
