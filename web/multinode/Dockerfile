# syntax=docker/dockerfile:1.4

ARG NODE_VERSION=24.11.1

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION} AS build

WORKDIR /work

COPY ./package*.json /work/
RUN --mount=type=cache,target=/root/.npm npm ci

COPY ./ /work/
RUN --mount=type=cache,target=/root/.npm npm run build

FROM scratch as export
COPY --from=build /work/dist /
