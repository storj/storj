# # build
# FROM golang:alpine AS build-env
# COPY . /go/src/storj.io/storj
# RUN cd /go/src/storj.io/storj && go install && ls al cmd/overlay && go build cmd/overlay/main.go
# 
# # final stage
# FROM alpine
# WORKDIR /app
# COPY --from=build-env /go/src/storj.io/storj/main /app/
# 
# ENTRYPOINT ./main

FROM alpine
WORKDIR /app
COPY ./main .
ENTRYPOINT ./main 
