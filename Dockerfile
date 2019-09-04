# Satellite UI static asset generation
FROM node:10.15.1 as satellite-ui
WORKDIR /app
COPY web/satellite/ /app
COPY web/marketing/ /app/marketing
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm rebuild node-sass
RUN npm run build

FROM golang:latest AS step1

# Set workdir to keep docker filesystem organized
WORKDIR /app

# Copy storj code into container
COPY . ./

RUN git reset --hard
RUN git checkout $(git describe --tags $(git rev-list --tags --max-count=1))

FROM golang:latest AS step2

WORKDIR /app

# Copy go.mod and go.sum first to improve caching
COPY --from=step1 app/go.mod app/go.sum ./

# Download all dependencies
RUN go mod download

# Copy storj code into container
COPY --from=step1 /app ./
COPY --from=satellite-ui /app/static ./static
COPY --from=satellite-ui /app/dist ./dist
COPY --from=satellite-ui /app/marketing ./marketing

RUN mkdir bin

# Now build
RUN go build -o bin/satellite storj.io/storj/cmd/satellite

EXPOSE 7777
EXPOSE 10100

ENTRYPOINT ["bin/satellite"]
CMD ["run", "--defaults", "dev"]
