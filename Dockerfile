FROM scratch
COPY storage-auth-proxy /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/storage-auth-proxy"]
