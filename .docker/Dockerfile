FROM busybox:1.31.1-glibc

COPY flywheel /
COPY buildInfo.json /
ENV GIN_MODE release
ENTRYPOINT ["/flywheel"]