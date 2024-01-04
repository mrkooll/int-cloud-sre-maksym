FROM busybox:1.36
RUN busybox --install /bin
COPY bin/sre-server /bin/sre-server
ENTRYPOINT ["/bin/sh", "-c", "/bin/sre-server -port $PORT -cacert $CA -cert $CERT -key $KEY -incluster"]
