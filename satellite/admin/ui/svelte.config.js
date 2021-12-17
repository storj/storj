// Copyright (C) 2021 Storj Labs, Inc.

import adapter from '@sveltejs/adapter-static';
import preprocess from 'svelte-preprocess';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// Consult https://github.com/sveltejs/svelte-preprocess
	// for more information about preprocessors
	preprocess: preprocess(),

	kit: {
		adapter: adapter(),

		// hydrate the <div id="svelte"> element in src/app.html
		target: '#svelte',

		ssr: false,
		vite: {
			resolve: {
				preserveSymlinks: true
			}
		}
	}
};

export default config;
