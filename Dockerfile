FROM docker.m.daocloud.io/library/golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /site-uploader .
FROM docker.m.daocloud.io/library/debian:bookworm-slim
RUN useradd --create-home --uid 10001 app && mkdir /data && chown app:app /data
WORKDIR /app
COPY --from=build /site-uploader .
COPY web ./web
USER app
EXPOSE 8080
CMD ["./site-uploader"]
