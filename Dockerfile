# BUILD STAGE
FROM golang:1.16-alpine AS build

ARG APP_VERSION="v5.0.0"
ARG CGO=1
ENV CGO_ENABLED=${CGO}
ENV GOOS=linux
ENV GO111MODULE=on

# gcc/g++ are required by sqlite driver
RUN apk update && apk add --no-cache gcc g++

WORKDIR /build
COPY . .
RUN go build -v -a -tags 'netgo' -ldflags '-w -X 'main.Version=${APP_VERSION}' -extldflags "-static"' -o chaos cmd/chaos/*

# FINAL STAGE
FROM golang:1.16-alpine
RUN apk update && apk add --no-cache gcc g++

MAINTAINER tiagorlampert@gmail.com

ENV DATABASE_NAME=chaos
ENV GIN_MODE=release

WORKDIR /
COPY --from=build /build/chaos /
COPY ./web /web
COPY ./client /client

EXPOSE 8080 3000
ENTRYPOINT ["/chaos"]