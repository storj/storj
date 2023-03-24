// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="permissions-select">
        <div
            class="permissions-select__toggle-container"
            aria-roledescription="select-permissions"
            @click.stop="toggleDropdown"
        >
            <p class="permissions-select__toggle-container__name">
                <span v-if="allPermissions">All</span>
                <span v-if="storedIsDownload && !allPermissions">Download </span>
                <span v-if="storedIsUpload && !allPermissions">Upload </span>
                <span v-if="storedIsList && !allPermissions">List </span>
                <span v-if="storedIsDelete && !allPermissions">Delete</span>
            </p>
            <ExpandIcon
                class="permissions-select__toggle-container__expand-icon"
                alt="Arrow down (expand)"
            />
        </div>
        <div v-if="isDropdownVisible" v-click-outside="closeDropdown" class="permissions-select__dropdown" @close="closeDropdown">
            <div class="permissions-select__dropdown__item">
                <input id="download" type="checkbox" name="download" :checked="storedIsDownload" @change="toggleIsDownload">
                <label class="permissions-select__dropdown__item__label" for="download">Download</label>
            </div>
            <div class="permissions-select__dropdown__item">
                <input id="upload" type="checkbox" name="upload" :checked="storedIsUpload" @change="toggleIsUpload">
                <label class="permissions-select__dropdown__item__label" for="upload">Upload</label>
            </div>
            <div class="permissions-select__dropdown__item">
                <input id="list" type="checkbox" name="list" :checked="storedIsList" @change="toggleIsList">
                <label class="permissions-select__dropdown__item__label" for="list">List</label>
            </div>
            <div class="permissions-select__dropdown__item">
                <input id="delete" type="checkbox" name="delete" :checked="storedIsDelete" @change="toggleIsDelete">
                <label class="permissions-select__dropdown__item__label" for="delete">Delete</label>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { ACCESS_GRANTS_MUTATIONS } from '@/store/modules/accessGrants';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

// @vue/component
@Component({
    components: {
        ExpandIcon,
    },
})
export default class PermissionsSelect extends Vue {
    public isLoading = true;

    /**
     * Toggles dropdown visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.PERMISSIONS);
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Sets is download permission.
     */
    public toggleIsDownload(): void {
        this.$store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_DOWNLOAD_PERMISSION);
    }

    /**
     * Sets is upload permission.
     */
    public toggleIsUpload(): void {
        this.$store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_UPLOAD_PERMISSION);
    }

    /**
     * Sets is list permission.
     */
    public toggleIsList(): void {
        this.$store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_LIST_PERMISSION);
    }

    /**
     * Sets is delete permission.
     */
    public toggleIsDelete(): void {
        this.$store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_DELETE_PERMISSION);
    }

    /**
     * Indicates if dropdown is visible.
     */
    public get isDropdownVisible(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.PERMISSIONS;
    }

    /**
     * Returns download permission from store.
     */
    public get storedIsDownload(): boolean {
        return this.$store.state.accessGrantsModule.isDownload;
    }

    /**
     * Returns upload permission from store.
     */
    public get storedIsUpload(): boolean {
        return this.$store.state.accessGrantsModule.isUpload;
    }

    /**
     * Returns list permission from store.
     */
    public get storedIsList(): boolean {
        return this.$store.state.accessGrantsModule.isList;
    }

    /**
     * Returns delete permission from store.
     */
    public get storedIsDelete(): boolean {
        return this.$store.state.accessGrantsModule.isDelete;
    }

    /**
     * Indicates if everything is allowed.
     */
    public get allPermissions(): boolean {
        return this.storedIsDownload && this.storedIsUpload && this.storedIsList && this.storedIsDelete;
    }
}
</script>

<style scoped lang="scss">
    .permissions-select {
        background-color: #fff;
        cursor: pointer;
        border-radius: 6px;
        border: 1px solid rgb(56 75 101 / 40%);
        font-family: 'font_regular', sans-serif;
        width: 100%;
        position: relative;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 15px 20px;
            width: calc(100% - 40px);
            border-radius: 6px;

            &__name {
                font-size: 16px;
                line-height: 21px;
                color: #384b65;
                margin: 0;
            }
        }

        &__dropdown {
            cursor: default;
            position: absolute;
            top: calc(100% + 5px);
            left: 0;
            z-index: 1;
            border-radius: 6px;
            border: 1px solid rgb(56 75 101 / 40%);
            background-color: #fff;
            padding: 10px 20px;
            width: calc(100% - 40px);
            box-shadow: 0 20px 34px rgb(10 27 44 / 28%);

            &__item {
                display: flex;
                align-items: center;
                cursor: pointer;

                &__label {
                    cursor: pointer;
                    font-size: 16px;
                    line-height: 26px;
                    color: #000;
                    margin-left: 15px;
                }
            }
        }
    }
</style>
