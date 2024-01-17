// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectRole } from '@/types/projectMembers';

/**
 * ProjectItemModel represents the view model for project items in the all projects dashboard.
 */
export class ProjectItemModel {
    public constructor(
        public id: string,
        public name: string,
        public description: string,
        public role: ProjectItemRole,
        public memberCount: number | null,
        public createdAt: Date,
        public storageUsed: string = '',
        public bandwidthUsed: string = '',
    ) {}
}

/**
 * ProjectItemRole represents the role of a user for a project item.
 */
export type ProjectItemRole = Exclude<ProjectRole, ProjectRole.InviteExpired>;

/**
 * PROJECT_ROLE_COLORS defines what colors project role tags should use.
 */
export const PROJECT_ROLE_COLORS: Record<ProjectRole, string> = {
    [ProjectRole.Member]: 'green',
    [ProjectRole.Owner]: 'secondary',
    [ProjectRole.Invited]: 'warning',
    [ProjectRole.InviteExpired]: 'error',
};
