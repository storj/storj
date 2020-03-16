// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { Store } from 'vuex';

import { Project } from '@/types/projects';

/**
 * ProjectOwning exposes method checking if user has his own project.
 */
export class ProjectOwning {
    public constructor(public store: Store<any>) {}

    public userHasOwnProject(): boolean {
        return this.store.state.projectsModule.projects.some((project: Project) => project.ownerId === this.store.getters.user.id);
    }
}
