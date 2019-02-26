FROM node:10.15.1

EXPOSE 8080
WORKDIR /app
COPY web/satellite/ /app
COPY web/satellite/entrypoint /entrypoint
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
ENTRYPOINT ["/entrypoint"]
