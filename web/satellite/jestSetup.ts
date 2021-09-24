// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { GlobalWithFetchMock } from 'jest-fetch-mock';

const customGlobal: GlobalWithFetchMock = global as GlobalWithFetchMock;

customGlobal.fetch = require('jest-fetch-mock');
customGlobal.fetchMock = customGlobal.fetch;

// Disallow warnings and errors from console.
customGlobal.console.warn = (message) => { throw new Error(message); };
customGlobal.console.error = (message) => { throw new Error(message); };
