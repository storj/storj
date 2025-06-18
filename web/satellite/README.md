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
# 1. Project structure:
- [**src**](./src "src") folder: contains main project components such as api, store, router, etc.
- [**static**](./static "static") folder: contains all project static resources such as images, fonts, pages.
- [**tests**](./tests "tests") folder: - contains project unit tests.
- **configuration files**.
###  src
- [api](./api "api")  folder: contains API for project modules such as auth, project, etc. We are using both [GraphQL](https://graphql.org/) and [HTTP](https://developer.mozilla.org/en-US/docs/Web/HTTP) implementations.
- [components](./components "components")  folder: contains hierarchy of vue single file components sorted thematically.
- [router](./router "router") folder: contains project browser locations structure file.
- [store](./store "store") folder: contains global state management file broken into modules.
- [types](./types "types") folder: contains project classes and types.
- [utils](./utils "utils")  folder: contains constants, plugins and utility files for formatting, validation, data transferring, etc.
- [views](./views "views")  folder: same as components, but for root ones.
- [App.vue](./src/App.vue "App.vue") root project component.
- [main.ts](./src/main.ts "main.ts") Vue instance initialization file. Here filters and declarations are placed. Also plugins, store and router are connecting to Vue instance.
### static
- [activation](./activation "activation") folder: contains page template that appears after account verification via email.
- [emails](./emails "emails") folder: contains all emails templates.
- [errors](./errors "errors") folder: contains 50x and 40x error pages templates.
- [fonts](./fonts "fonts") folder: contains Inter font sets in ttf format.
- [images](./images "images") folder: contains illustrations.
- [reports](./reports "reports") folder: contains usage report table template.
### tests
- [unit](./unit "unit") folder: contains project unit tests.
### Configuration files
- **.env**: file for environment level variables.
- **.gitignore**: folders, files and extensions which are ignored for git.
- **babel.config.js**: [babel](https://babeljs.io/) configuration for javascript transcompilation.
- **index.html**: DOM entry point.
- **jestSetup.ts**: [jest](https://jestjs.io/) configuration for unit testing.
- **package.json**: file holds various metadata relevant to the project such as version, dependencies, scripts and configurations.
- **tsconfig.json**: holds [TypeScript](https://www.typescriptlang.org/) configurations.
- **tslint.json**: holds [TypeScript](https://www.typescriptlang.org/) linter configurations.
- **vue.config.js**: holds [Vue](https://vuejs.org/) configurations.
