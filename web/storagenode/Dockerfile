FROM debian:bookworm AS build
ENV NODE_MAJOR=20
RUN echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list
RUN apt-get update && \
  DEBIAN_FRONTEND="noninteractive" apt-get install -y nodejs npm && \
  npm install -g npm@10.4.0 && \
  npm install -g pnpm@v8
ADD web/storagenode /work
WORKDIR /work
RUN npm ci && npm run build

FROM debian:bookworm as ui
COPY --from=build /work/dist /var/www/html