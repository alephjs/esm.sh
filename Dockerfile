# Build Stage
FROM golang:1.23-alpine AS build-stage

ENV ESM_SH_REPO https://github.com/esm-dev/esm.sh
ENV ESM_SH_VERSION v136

RUN apk update && apk add --no-cache git
RUN git clone --branch $ESM_SH_VERSION --depth 1 $ESM_SH_REPO /tmp/esm.sh

WORKDIR /tmp/esm.sh
RUN CGO_ENABLED=0 GOOS=linux go build -o esmd main.go

# Release Stage
FROM node:22-alpine AS release-stage

RUN apk update && apk add --no-cache git git-lfs libcap-utils
RUN git lfs install
RUN npm i -g pnpm

COPY --from=build-stage /tmp/esm.sh/esmd /bin/esmd
RUN setcap cap_net_bind_service=ep /bin/esmd
RUN chown node:node /bin/esmd

USER node
WORKDIR /tmp
EXPOSE 80
CMD ["esmd"]
