FROM alpine:3.19.0
RUN apk add bash
COPY bin/sre-server /bin/sre-server
ENTRYPOINT ["/bin/sh", "-c", "/bin/sre-server -port $PORT -cacert $CA -cert $CERT -key $KEY -incluster"]
