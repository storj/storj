// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectMemberCursor, ProjectMembersApi, ProjectMembersPage } from '@/types/projectMembers';

/**
 * Mock for ProjectMembersApi
 */
export class ProjectMembersApiMock implements ProjectMembersApi {
    public cursor: ProjectMemberCursor;
    public page: ProjectMembersPage;

    public setMockPage(page: ProjectMembersPage): void {
        this.page = page;
    }

    add(projectId: string, emails: string[]): Promise<void> {
        throw new Error('not implemented');
    }

    delete(projectId: string, emails: string[]): Promise<void> {
        throw new Error('not implemented');
    }

    get(projectId: string, cursor: ProjectMemberCursor): Promise<ProjectMembersPage> {
        return Promise.resolve(this.page);
    }
}
