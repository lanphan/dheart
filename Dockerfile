FROM golang:1.16-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git
RUN apk add openssh

ENV GO111MODULE=on

RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
RUN mkdir /root/.ssh && echo "StrictHostKeyChecking no " > /root/.ssh/config

# # Though the id_rsa file is removed at the end of this docker build, it's still dangerous to include
# # id_rsa in the build file since docker build steps are cached. Only do this while our repos are in
# # private mode.
ADD build/id_rsa /root/.ssh/id_rsa

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY tmp/ .
RUN go build -o main

RUN rm /root/.ssh/id_rsa

CMD ["ls", "/root/.ssh/id_rsa"]
# RUN go mod download