// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="edit-project">
        <div class="edit-project__selection-area" :class="{ active: isDropdownShown }" @click.stop.prevent="toggleDropdown">
            <h1 class="edit-project__selection-area__name">{{ projectName }}</h1>
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

@Component({
    components: {
        DotsImage,
        EditImage,
    },
})
export default class EditProjectDropdown extends Vue {
    public isDropdownShown: boolean = false;

    /**
     * Returns selected project's name.
     */
    public get projectName(): string {
        return this.$store.getters.selectedProject.name;
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
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }
}
</script>

<style scoped lang="scss">
    .edit-project {
        font-family: 'font_regular', sans-serif;
        width: 190px;
        position: relative;
        margin: 0 0 30px 13px;

        &__selection-area {
            width: calc(100% - 25px);
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px 15px 10px 10px;
            border-radius: 6px;
            cursor: pointer;

            &__name {
                font-family: 'font_bold', sans-serif;
                font-size: 18px;
                line-height: 22px;
                color: #000;
                margin: 0 5px 0 0;
                word-break: break-all;
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
            width: 190px;
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
                }
            }
        }
    }

    .active {
        background-color: rgba(245, 246, 250, 0.7);
    }
</style>
