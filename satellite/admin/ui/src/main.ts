// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/*
 * Svelte App entry point.
 */

import App from "./App.svelte";

const app = new App({
  target: document.body,
});

export default app;
