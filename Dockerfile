FROM scratch
COPY zot-docker-proxy /
ENTRYPOINT ["/zot-docker-proxy"]
