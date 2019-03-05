# storj-dev-panel

## Project setup
```
npm install
```

### Compiles and hot-reloads for development
```
npm run serve
```

### Compiles and minifies for production
```
npm run build
```

### Run your tests
```
npm run test
```

### Lints and fixes files
```
npm run lint
```

### Run your unit tests
```
npm run test:unit
```

### Build docker container
From root of the repository, run:
```
make satellite-ui-image
```

### Run docker container
```
docker run -p 8080:8080 storjlabs/satellite-ui:latest
```
