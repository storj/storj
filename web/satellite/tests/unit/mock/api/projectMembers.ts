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

    add(_projectId: string, _emails: string[]): Promise<void> {
        throw new Error('not implemented');
    }

    delete(_projectId: string, _emails: string[]): Promise<void> {
        throw new Error('not implemented');
    }

    get(_projectId: string, _cursor: ProjectMemberCursor): Promise<ProjectMembersPage> {
        return Promise.resolve(this.page);
    }
}
