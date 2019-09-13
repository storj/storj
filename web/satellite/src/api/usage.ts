// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import gql from 'graphql-tag';

import { BaseGql } from '@/api/baseGql';
import { ProjectUsage, UsageApi } from '@/types/usage';
import apollo from '@/utils/apollo';

/**
 * Exposes all project-usage-related functionality
 */
export class ProjectUsageApiGql extends BaseGql implements UsageApi {
    /**
     * Fetch usage
     *
     * @returns ProjectUsage
     * @throws Error
     */
    public async get(projectId: string, since: Date, before: Date): Promise<ProjectUsage> {
        const query = `
            query($projectId: String!, $since: DateTime!, $before: DateTime!) {
                project(id: $projectId) {
                    usage(since: $since, before: $before) {
                        storage,
                        egress,
                        objectCount,
                        since,
                        before
                    }
                }
            }`;

        const variables = {
            projectId,
            since,
            before,
        };

        const response = await this.query(query, variables);

        return response.data.project.usage;
    }
}
