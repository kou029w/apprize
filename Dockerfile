# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

FROM golang:1.26@sha256:68cb6d68bed024785b69195b89af7ac7a444f27791435f98647edff595aa0479 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/apprize

FROM gcr.io/distroless/base-debian12:nonroot@sha256:7a75a36f4bec82a7542c64195e402907486f9a4dd2f8797a976aa0cf31cfb470
WORKDIR /app

COPY --from=build /out/apprize /usr/local/bin/apprize

ENV APPRIZE_BIND=:8000
EXPOSE 8000

ENTRYPOINT ["/usr/local/bin/apprize"]
