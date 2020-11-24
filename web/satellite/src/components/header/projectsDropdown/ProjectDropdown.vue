// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dropdown">
        <div class="project-dropdown__wrap">
            <div class="project-dropdown__wrap__choice" @click.prevent.stop="closeDropdown">
                <div class="project-dropdown__wrap__choice__mark-container">
                    <SelectionIcon
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

import SelectionIcon from '@/../static/images/header/selection.svg';

import { RouteConfig } from '@/router';
import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        SelectionIcon,
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
        LocalData.setSelectedProjectId(projectID);
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
        z-index: 1120;
        left: 0;
        top: 50px;
        box-shadow: 0 20px 34px rgba(10, 27, 44, 0.28);
        border-radius: 6px;
        background-color: #fff;
        padding-top: 6px;

        &__wrap {
            overflow-y: scroll;
            height: auto;
            min-width: 300px;
            max-height: 250px;
            background-color: #fff;
            border-radius: 6px;
            font-family: 'font_regular', sans-serif;

            &__choice {
                width: auto;
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 0 25px;

                &__selected,
                &__unselected {
                    margin: 12px 0;
                    font-size: 14px;
                    line-height: 20px;
                    color: #1b2533;
                    word-break: break-all;
                }

                &__selected {
                    font-family: 'font_bold', sans-serif;
                }

                &__unselected {
                    padding-left: 22px;
                }

                &:hover {
                    background-color: #f5f6fa;

                    .project-dropdown__wrap__choice__unselected {
                        font-family: 'font_bold', sans-serif;
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
                    color: #0068dc;
                    font-size: 14px;
                }
            }
        }
    }
</style>
