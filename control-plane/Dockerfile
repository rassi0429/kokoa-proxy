FROM golang:1.22 AS build

WORKDIR /src
COPY control-plane/go.mod control-plane/go.sum ./ 
RUN go mod download

COPY control-plane/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kokoa-cp ./cmd/kokoa-cp

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
RUN mkdir -p /data && chown appuser:appuser /data
WORKDIR /app
COPY --from=build /out/kokoa-cp /usr/local/bin/kokoa-cp
USER appuser

ENV CP_LISTEN_ADDR=:8080
ENV CP_DB_PATH=/data/kokoa.db

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["kokoa-cp"]
