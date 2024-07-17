FROM busybox:1.36.1-glibc

COPY flywheel /
COPY buildInfo.json /
ENV GIN_MODE release
ENTRYPOINT ["/flywheel"]
