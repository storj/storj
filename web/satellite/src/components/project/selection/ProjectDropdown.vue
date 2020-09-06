// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dropdown">
        <div class="project-dropdown__wrap">
            <div class="project-dropdown__wrap__choice" @click.prevent.stop="closeDropdown">
                <div class="project-dropdown__wrap__choice__mark-container">
                    <ProjectSelectionIcon
                        class="project-dropdown__wrap__choice__mark-container__image"
                    />
                </div>
                <p class="project-dropdown__wrap__choice__selected">
                    {{ selectedProject.name }}
                </p>
            </div>
            <div
                class="project-dropdown__wrap__choice"
                @click.prevent.stop="onProjectSelected(project.id)"
                v-for="project in projects"
                :key="project.id"
            >
                <p class="project-dropdown__wrap__choice__unselected">{{ project.name }}</p>
            </div>
        </div>
        <div class="project-dropdown__create-project" v-if="isCreateProjectButtonShown" @click.stop="onCreateProjectsClick">
            <div class="project-dropdown__create-project__border"/>
            <div class="project-dropdown__create-project__button-area">
                <p class="project-dropdown__create-project__button-area__text">Create Projects</p>
                <p class="project-dropdown__create-project__button-area__arrow">-></p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelectionIcon from '@/../static/images/header/projectSelection.svg';

import { RouteConfig } from '@/router';
import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        ProjectSelectionIcon,
    },
})
export default class ProjectDropdown extends Vue {
    private FIRST_PAGE = 1;

    /**
     * Fetches all project related information.
     * @param projectID
     */
    public async onProjectSelected(projectID: string): Promise<void> {
        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
        this.closeDropdown();

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(`Unable to select project. ${error.message}`);
        }
    }

    /**
     * Returns projects list from store.
     */
    public get projects(): Project[] {
        return this.$store.getters.projectsWithoutSelected;
    }

    /**
     * Returns selected project from store.
     */
    public get selectedProject(): Project {
        return this.$store.getters.selectedProject;
    }

    /**
     * Indicates if create project button is shown.
     */
    public get isCreateProjectButtonShown(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectButtonShown;
    }

    /**
     * Redirects to create project page.
     */
    public onCreateProjectsClick(): void {
        this.$router.push(RouteConfig.CreateProject.path);
        this.closeDropdown();
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$emit('close');
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin: 0;
        width: 0;
    }

    .project-dropdown {
        position: absolute;
        left: -3px;
        top: 60px;
        border: 1px solid #c5cbdb;
        box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
        border-radius: 6px;
        background-color: #fff;

        &__wrap {
            width: auto;
            overflow-y: scroll;
            height: auto;
            min-width: 195px;
            max-height: 240px;
            background-color: #fff;
            border-radius: 6px;
            font-family: 'font_regular', sans-serif;

            &__choice {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 0 25px;

                &__selected,
                &__unselected {
                    margin: 12px 0;
                    font-size: 14px;
                    line-height: 20px;
                    color: #7e8b9c;
                    word-break: break-all;
                }

                &__selected {
                    font-family: 'font_bold', sans-serif;
                    color: #494949;
                }

                &:hover {
                    background-color: #f2f2f6;

                    .project-dropdown__wrap__choice__unselected {
                        color: #354049;
                    }
                }

                &__mark-container {
                    width: 10px;
                    margin-right: 12px;

                    &__image {
                        object-fit: cover;
                    }
                }
            }
        }

        &__create-project {

            &__border {
                border-top: 1px solid #c7cdd2;
                width: 90%;
                float: right;
            }

            &__button-area {
                display: flex;
                justify-content: space-between;
                align-items: center;
                width: calc(100% - 50px);
                padding: 5px 25px;

                &__text,
                &__arrow {
                    color: #2683ff;
                    font-size: 14px;
                }
            }
        }
    }
</style>
