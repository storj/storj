# Admin UI

## Implementation details

This is a project based on the [Svelte](https://svelte.dev) [template for apps](https://github.com/sveltejs/template).

The project templated was converted to Typescript following the instructions on its README.

The Web App is currently straightforward as we specified that v1 would be.

The v1 is just a simple web page that exposes the Admin API through some forms and allow to a call the API without needing to use some HTTP REST clients (e.g. Postman, cURL, etc.).
It doesn't offer any user authentication; the user has to know the API authorization token for using it.


The UI has a set of Svelte components that collaborate together to render an HTML form with input elements from the Admin API client.
The Svelte components expect some values of a certain Typescript interfaces, types, and classes, for being able to dynamically render the HTML form and elements.

Each source has a brief doc comment about its functionality.

## Development

Install the dependencies...

```bash
npm install
```

...then start [Rollup](https://rollupjs.org):

```bash
npm run dev
```

Navigate to [localhost:5000](http://localhost:5000). You should see your app running.

By default, the server will only respond to requests from localhost. To allow connections from other computers, edit the `sirv` commands in package.json to include the option `--host 0.0.0.0`.


## Building for production mode

To create an optimised version of the app:

```bash
npm run build
```
