FROM node:10.15.1 as satellite-ui

WORKDIR /app
COPY web/satellite/ /app
COPY web/satellite/entrypoint /entrypoint
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build
ENTRYPOINT ["/entrypoint"]
