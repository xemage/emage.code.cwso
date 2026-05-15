FROM alpine:3.21

RUN apk add --no-cache bash

COPY host_probe.sh /usr/local/bin/host_probe.sh
RUN chmod 0755 /usr/local/bin/host_probe.sh

ENTRYPOINT ["/usr/local/bin/host_probe.sh"]
