// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import createFetchMock from 'vitest-fetch-mock';
import { vi } from 'vitest';

const fetchMocker = createFetchMock(vi);

fetchMocker.enableMocks();
