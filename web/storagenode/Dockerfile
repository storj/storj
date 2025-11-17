FROM node:24.11.1 AS build
ADD web/storagenode /work
WORKDIR /work
RUN npm ci && npm run build

FROM debian:bookworm as ui
COPY --from=build /work/dist /var/www/html
