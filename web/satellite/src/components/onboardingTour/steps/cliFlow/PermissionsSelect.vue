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

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ACCESS_GRANTS_MUTATIONS } from '@/store/modules/accessGrants';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useStore } from '@/utils/hooks';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

const store = useStore();

const isLoading = ref<boolean>(true);

/**
 * Indicates if dropdown is visible.
 */
const isDropdownVisible = computed((): boolean => {
    return store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.PERMISSIONS;
});

/**
 * Returns download permission from store.
 */
const storedIsDownload = computed((): boolean => {
    return store.state.accessGrantsModule.isDownload;
});

/**
 * Returns upload permission from store.
 */
const storedIsUpload = computed((): boolean => {
    return store.state.accessGrantsModule.isUpload;
});

/**
 * Returns list permission from store.
 */
const storedIsList = computed((): boolean => {
    return store.state.accessGrantsModule.isList;
});

/**
 * Returns delete permission from store.
 */
const storedIsDelete = computed((): boolean => {
    return store.state.accessGrantsModule.isDelete;
});

/**
 * Indicates if everything is allowed.
 */
const allPermissions = computed((): boolean => {
    return storedIsDownload.value && storedIsUpload.value && storedIsList.value && storedIsDelete.value;
});

/**
 * Toggles dropdown visibility.
 */
function toggleDropdown(): void {
    store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.PERMISSIONS);
}

/**
 * Closes dropdown.
 */
function closeDropdown(): void {
    store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
}

/**
 * Sets is download permission.
 */
function toggleIsDownload(): void {
    store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_DOWNLOAD_PERMISSION);
}

/**
 * Sets is upload permission.
 */
function toggleIsUpload(): void {
    store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_UPLOAD_PERMISSION);
}

/**
 * Sets is list permission.
 */
function toggleIsList(): void {
    store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_LIST_PERMISSION);
}

/**
 * Sets is delete permission.
 */
function toggleIsDelete(): void {
    store.commit(ACCESS_GRANTS_MUTATIONS.TOGGLE_IS_DELETE_PERMISSION);
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
