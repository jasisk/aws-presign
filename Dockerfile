# Start by building the application.
FROM golang:1.11beta3 as build

WORKDIR /go/src/app
COPY aws-presign.go go.mod go.sum ./

RUN go get -v ./...
RUN go install -v ./...

# Now copy it into our base image.
FROM gcr.io/distroless/base
COPY --from=build /go/bin/app /
ENTRYPOINT ["/app"]
