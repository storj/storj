// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectUsage, UsageApi } from '@/types/usage';

/**
 * Mock for UsageApi
 */
export class ProjectUsageMock implements UsageApi {
    get(projectId: string, since: Date, before: Date): Promise<ProjectUsage> {
        throw new Error('Method not implemented.');
    }
}
