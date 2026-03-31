FROM alpine:3.21

RUN apk add --no-cache openssh-server tmux bash

RUN ssh-keygen -A

RUN adduser -D -s /bin/bash demo && \
    mkdir -p /home/demo/.ssh && \
    chmod 700 /home/demo/.ssh

COPY id_ed25519.pub /home/demo/.ssh/authorized_keys
RUN chmod 600 /home/demo/.ssh/authorized_keys && \
    chown -R demo:demo /home/demo/.ssh

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22

ENTRYPOINT ["/entrypoint.sh"]
