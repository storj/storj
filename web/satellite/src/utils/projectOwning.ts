// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import store from '@/store';
import { Project } from '@/types/projects';

/**
 * ProjectOwning exposes method checking if user has his own project.
 */
export class ProjectOwning {
    public static userHasOwnProject(): boolean {
        return store.state.projectsModule.projects.some((project: Project) => project.ownerId === store.getters.user.id);
    }
}
