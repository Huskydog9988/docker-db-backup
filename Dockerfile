# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS BUILD
ARG Release=dev

RUN apk add build-base alpine-sdk

WORKDIR /app

# RUN apk update && apk add --no-cache musl-dev gcc build-base

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.* ./
RUN go mod download && go mod verify

# don't look for dynamic libraries (like c libraries) since we're building a static binary
ENV CGO_ENABLED false

# fix for sqlite3 not building on alpine
# https://github.com/mattn/go-sqlite3/issues/1164#issuecomment-1635253695
ENV CGO_CFLAGS "-D_LARGEFILE64_SOURCE"

COPY . ./
# based on https://www.cloudbees.com/blog/building-minimal-docker-containers-for-go-applications
RUN go build -a -installsuffix cgo -v -o /app/server .

FROM alpine as deployment

WORKDIR /app

COPY --from=BUILD /app/server /app/server

ENTRYPOINT ["/app/server"]