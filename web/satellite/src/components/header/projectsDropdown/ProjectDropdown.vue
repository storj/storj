// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dropdown">
        <div v-if="isLoading" class="project-dropdown__loader-container">
            <VLoader
                width="30px"
                height="30px"
            />
        </div>
        <div v-else class="project-dropdown__wrap">
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
                v-for="project in projects"
                :key="project.id"
                class="project-dropdown__wrap__choice"
                @click.prevent.stop="onProjectSelected(project.id)"
            >
                <p class="project-dropdown__wrap__choice__unselected">{{ project.name }}</p>
            </div>
        </div>
        <div class="project-dropdown__create-project" @click.stop="onProjectsLinkClick">
            <div class="project-dropdown__create-project__border" />
            <div class="project-dropdown__create-project__button-area">
                <p class="project-dropdown__create-project__button-area__text">Manage Projects</p>
                <p class="project-dropdown__create-project__button-area__arrow">-></p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';

import SelectionIcon from '@/../static/images/header/selection.svg';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';

// @vue/component
@Component({
    components: {
        SelectionIcon,
        VLoader,
    },
})
export default class ProjectDropdown extends Vue {
    @Prop({ default: false })
    public readonly isLoading: boolean;

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

        if (this.isBucketsView) {
            await this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
            await this.$router.push({name: RouteConfig.Buckets.name}).catch(() => {return; });
        }

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
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
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$emit('close');
    }

    /**
     * Route to projects list page.
     */
    public onProjectsLinkClick(): void {
        if (this.$route.name !== RouteConfig.ProjectsList.name) {
            this.$router.push(RouteConfig.ProjectsList.path);
        }

        this.$emit('close');
    }

    /**
     * Indicates if current route is objects view.
     */
    private get isBucketsView(): boolean {
        return this.$route.path.includes(RouteConfig.Buckets.path);
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
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
        border-radius: 6px;
        background-color: #fff;
        padding-top: 6px;
        min-width: 300px;

        &__loader-container {
            margin: 10px 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }

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
                padding: 15px 25px;

                &__text,
                &__arrow {
                    color: #0068dc;
                    font-size: 14px;
                }
            }
        }
    }
</style>
