# checkov:skip=CKV_DOCKER_2: Ensure that HEALTHCHECK instructions have been added to container images

FROM curlimages/curl:8.12.1 AS bins

USER root

RUN if [ "$(uname -m)" = "x86_64" ]; then \
      curl -sfLo /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_x86_64; \
      curl  -sfLo /tmp/gh-cli.tar.gz https://github.com/cli/cli/releases/download/v2.61.0/gh_2.61.0_linux_amd64.tar.gz; \
      tar -xzf /tmp/gh-cli.tar.gz -C /usr/local/bin --strip-components=2 gh_2.61.0_linux_amd64/bin/gh; \
    elif [ "$(uname -m)" = "aarch64" ]; then \
      curl -sfLo /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_aarch64; \
      curl  -sfLo /tmp/gh-cli.tar.gz https://github.com/cli/cli/releases/download/v2.61.0/gh_2.61.0_linux_arm64.tar.gz; \
      tar -xzf /tmp/gh-cli.tar.gz -C /usr/local/bin --strip-components=2 gh_2.61.0_linux_arm64/bin/gh; \
    else \
      echo "Unsupported architecture"; exit 1; \
    fi && \
    chmod +x /usr/local/bin/dumb-init

FROM alpine:3

COPY --from=bins /usr/local/bin/dumb-init /usr/local/bin/dumb-init
COPY --from=bins /usr/local/bin/gh /usr/local/bin/gh

RUN apk add --update --no-cache jq

USER nobody:nobody

WORKDIR /app

COPY main.sh /usr/local/bin/main.sh
COPY *.json.tpl /

ENTRYPOINT ["/usr/local/bin/dumb-init", "--", "/usr/local/bin/main.sh"]
