FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/DynamycSound/unlapse/internal/server.Version=${VERSION}" \
    -o /unlapse ./cmd/unlapse

FROM scratch
COPY --from=build /unlapse /unlapse
ENV UNLAPSE_DATA=/data/unlapse-data.json
ENV UNLAPSE_ADDR=0.0.0.0:8275
VOLUME /data
EXPOSE 8275
ENTRYPOINT ["/unlapse"]
