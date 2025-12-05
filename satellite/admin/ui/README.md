# Admin UI

## Implementation details

This is a project based on the [Sveltekit](https://kit.svelte.dev).

The project is set up with Typescript.

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

...then run the development server with autoreload on changes

```bash
npm run dev
```

Navigate to [localhost:3000](http://localhost:3000). You should see your app running.

## Building for production mode

To create an optimized version of the app:

```bash
npm run build
```
