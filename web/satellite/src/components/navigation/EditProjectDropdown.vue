// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="edit-project">
        <div class="edit-project__selection-area" :class="{ active: isDropdownShown, 'on-edit': isEditPage }" @click.stop.prevent="toggleDropdown">
            <h1 class="edit-project__selection-area__name" :title="projectName">{{ projectName }}</h1>
            <DotsImage class="edit-project__selection-area__image"/>
        </div>
        <div class="edit-project__dropdown" v-if="isDropdownShown" v-click-outside="closeDropdown">
            <div class="edit-project__dropdown__choice" @click.stop.prevent="onEditProjectClick">
                <EditImage/>
                <p class="edit-project__dropdown__choice__label">Edit Details</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DotsImage from '@/../static/images/navigation/dots.svg';
import EditImage from '@/../static/images/navigation/edit.svg';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        DotsImage,
        EditImage,
    },
})
export default class EditProjectDropdown extends Vue {
    /**
     * Returns selected project's name.
     */
    public get projectName(): string {
        return this.$store.getters.selectedProject.name;
    }

    /**
     * Indicates if dropdown is shown.
     */
    public get isDropdownShown(): string {
        return this.$store.state.appStateModule.appState.isEditProjectDropdownShown;
    }

    /**
     * Indicates if current route name equals "edit project details" route name.
     */
    public get isEditPage(): boolean {
        return this.$route.name === RouteConfig.EditProjectDetails.name;
    }

    /**
     * Redirects to edit project details page.
     */
    public onEditProjectClick(): void {
        this.closeDropdown();
        this.$router.push(RouteConfig.EditProjectDetails.path);
    }

    /**
     * Toggles dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROJECT_DROPDOWN);
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        if (!this.isDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }
}
</script>

<style scoped lang="scss">
    .edit-project {
        font-family: 'font_regular', sans-serif;
        position: relative;
        width: 185px;
        margin: 0 0 30px 15px;

        &__selection-area {
            width: calc(100% - 15px);
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px;
            border-radius: 6px;
            cursor: pointer;

            &__name {
                font-family: 'font_bold', sans-serif;
                font-size: 18px;
                line-height: 22px;
                color: #000;
                margin: 0 5px 0 0;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }

            &__image {
                min-width: 3px;
            }
        }

        &__dropdown {
            position: absolute;
            top: calc(100% + 5px);
            left: 0;
            background-color: #fff;
            box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
            border-radius: 6px;
            padding: 5px 0;
            width: calc(100% + 5px);
            z-index: 1;

            &__choice {
                background-color: #fff;
                width: calc(100% - 32px);
                padding: 10px 16px;
                cursor: pointer;
                display: flex;
                align-items: center;

                &__label {
                    margin: 0 0 0 10px;
                    font-weight: 500;
                    font-size: 14px;
                    line-height: 19px;
                    color: #354049;
                }

                &:hover {
                    background-color: #f5f5f7;

                    .edit-project__dropdown__choice__label {
                        font-weight: unset;
                        font-family: 'font_bold', sans-serif;
                    }
                }
            }
        }
    }

    .active {
        background-color: #fff;
    }

    .on-edit {
        background-color: #0068dc;

        .edit-project__selection-area__name {
            color: #fff;
        }

        .edit-dots-svg-path {
            fill: #fff;
        }
    }
</style>
