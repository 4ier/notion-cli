FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY notion /usr/local/bin/notion
ENTRYPOINT ["notion"]
