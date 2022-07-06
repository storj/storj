// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

/** @type {import('@sveltejs/kit').Handle} */
export async function handle({ event, resolve }) {
	const response = await resolve(event, {
		ssr: false
	});

	return response;
}
