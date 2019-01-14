
# build
FROM golang:alpine as build-env
ADD . /go/src/storj.io/storj
WORKDIR /go/src/storj.io/storj/cmd/storagenode
# dependencies + binary
RUN apk add --no-cache gcc musl-dev && CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o storagenode . && mkdir config

# final stage
FROM alpine

EXPOSE 28967
EXPOSE 7777

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/

ENTRYPOINT ["./storagenode", "run", "--config-dir=config"]