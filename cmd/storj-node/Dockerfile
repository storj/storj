# build
FROM golang:alpine AS build-env
ADD . /go/src/research/lang/storj-node-go/
RUN cd /go/src/research/lang/storj-node-go/ && go build -o main

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/research/lang/storj-node-go/main /app/

ENTRYPOINT ./main
