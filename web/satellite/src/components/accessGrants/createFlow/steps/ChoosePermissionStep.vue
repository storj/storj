// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="choose">
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Permissions]">
            <template #functional>
                <div class="choose__toggles">
                    <Toggle
                        id="allPermissions"
                        :checked="selectedPermissions.length === 4"
                        label="All"
                        :on-check="() => onSelectPermission(Permission.All)"
                        :on-show-hide-all="togglePermissionsVisibility"
                        :all-shown="allPermissionsShown"
                    />
                    <template v-if="allPermissionsShown">
                        <Toggle
                            :checked="selectedPermissions.includes(Permission.Read)"
                            label="Read"
                            :on-check="() => onSelectPermission(Permission.Read)"
                        />
                        <Toggle
                            :checked="selectedPermissions.includes(Permission.Write)"
                            label="Write"
                            :on-check="() => onSelectPermission(Permission.Write)"
                        />
                        <Toggle
                            :checked="selectedPermissions.includes(Permission.List)"
                            label="List"
                            :on-check="() => onSelectPermission(Permission.List)"
                        />
                        <Toggle
                            :checked="selectedPermissions.includes(Permission.Delete)"
                            label="Delete"
                            :on-check="() => onSelectPermission(Permission.Delete)"
                        />
                    </template>
                </div>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Buckets]">
            <template #functional>
                <Toggle
                    id="allBuckets"
                    :checked="selectedBuckets.length === 0"
                    label="All"
                    :disabled="selectedBuckets.length === 0"
                    :on-check="selectAllBuckets"
                    :on-show-hide-all="toggleBucketsVisibility"
                    :all-shown="searchBucketsShown"
                />
                <template v-if="searchBucketsShown">
                    <div v-if="selectedBuckets.length" class="choose__selected-container">
                        <div v-for="bucket in selectedBuckets" :key="bucket" class="choose__selected-container__item">
                            <p class="choose__selected-container__item__label">{{ bucket }}</p>
                            <CloseIcon tabindex="0" @click="() => onUnselectBucket(bucket)" @keyup.space="() => onUnselectBucket(bucket)" />
                        </div>
                    </div>
                    <div class="choose__search-container">
                        <SearchIcon />
                        <input v-model="searchQuery" placeholder="Search by name">
                    </div>
                    <div v-if="searchQuery" class="choose__bucket-results">
                        <template v-if="bucketsList.length">
                            <p
                                v-for="bucket in bucketsList"
                                :key="bucket"
                                tabindex="0"
                                class="choose__bucket-results__item"
                                @click="() => selectBucket(bucket)"
                                @keyup.space="() => selectBucket(bucket)"
                            >
                                {{ bucket }}
                            </p>
                        </template>
                        <template v-else>
                            <p class="choose__bucket-results__empty">No Buckets found.</p>
                        </template>
                    </div>
                </template>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.EndDate]">
            <template #functional>
                <div class="choose__date-selection">
                    <p
                        v-if="!settingDate && !notAfter"
                        tabindex="0"
                        class="choose__date-selection__label"
                        @click="toggleSettingDate"
                        @keyup.space="toggleSettingDate"
                    >
                        Add Date (optional)
                    </p>
                    <EndDateSelection
                        v-else
                        :set-not-after="onSetNotAfter"
                        :not-after-label="notAfterLabel"
                        :set-not-after-label="onSetNotAfterLabel"
                    />
                </div>
            </template>
        </ContainerWithIcon>
        <ButtonsContainer>
            <template #leftButton>
                <VButton
                    label="Back"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onBack"
                    :is-white="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Continue ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="handleContinue"
                    :is-disabled="isButtonDisabled"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import {
    FUNCTIONAL_CONTAINER_ICON_AND_TITLE,
    FunctionalContainer,
    Permission,
} from '@/types/createAccessGrant';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import ContainerWithIcon from '@/components/accessGrants/createFlow/components/ContainerWithIcon.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import EndDateSelection from '@/components/accessGrants/createFlow/components/EndDateSelection.vue';
import Toggle from '@/components/accessGrants/createFlow/components/Toggle.vue';
import VButton from '@/components/common/VButton.vue';

import SearchIcon from '@/../static/images/common/search.svg';
import CloseIcon from '@/../static/images/accessGrants/newCreateFlow/close.svg';

const props = withDefaults(defineProps<{
    selectedPermissions: Permission[];
    onSelectPermission: (type: Permission) => void;
    selectedBuckets: string[];
    onSelectBucket: (bucket: string) => void;
    onUnselectBucket: (bucket: string) => void;
    onSelectAllBuckets: () => void;
    onSetNotAfter: (date: Date | undefined) => void;
    onSetNotAfterLabel: (label: string) => void;
    notAfterLabel: string;
    onBack: () => void;
    onContinue: () => void;
    notAfter?: Date;
}>(), {
    notAfter: undefined,
});

const bucketsStore = useBucketsStore();
const notify = useNotify();

const allPermissionsShown = ref<boolean>(false);
const searchBucketsShown = ref<boolean>(false);
const settingDate = ref<boolean>(false);
const searchQuery = ref<string>('');

/**
 * Indicates if button should be disabled.
 */
const isButtonDisabled = computed((): boolean => {
    return !props.selectedPermissions.length;
});

/**
 * Returns stored bucket names list filtered by search string.
 */
const bucketsList = computed((): string[] => {
    const NON_EXIST_INDEX = -1;
    const buckets: string[] = bucketsStore.state.allBucketNames;

    return buckets.filter((name: string) => {
        return name.indexOf(searchQuery.value.toLowerCase()) !== NON_EXIST_INDEX && !props.selectedBuckets.includes(name);
    });
});

/**
 * Selects bucket and clears search.
 */
function selectBucket(bucket: string): void {
    props.onSelectBucket(bucket);
    searchQuery.value = '';
}

/**
 * Handles continue button click.
 */
function handleContinue(): void {
    if (props.notAfter) {
        const now = new Date();
        now.setHours(0, 0, 0, 0);

        if (props.notAfter.getTime() < now.getTime()) {
            notify.error('End date must be later or equal to today.', AnalyticsErrorEventSource.CREATE_AG_MODAL);
            return;
        }
    }

    props.onContinue();
}

/**
 * Selects all buckets and clears search.
 */
function selectAllBuckets(): void {
    props.onSelectAllBuckets();
    searchQuery.value = '';
}

/**
 * Toggles end date selection visibility.
 */
function toggleSettingDate(): void {
    settingDate.value = true;
}

/**
 * Toggles full list of permissions visibility.
 */
function togglePermissionsVisibility(): void {
    allPermissionsShown.value = !allPermissionsShown.value;
}

/**
 * Toggles bucket search/select visibility.
 */
function toggleBucketsVisibility(): void {
    searchBucketsShown.value = !searchBucketsShown.value;
}
</script>

<style lang="scss" scoped>
.choose {
    font-family: 'font_regular', sans-serif;

    &__toggles {
        display: flex;
        flex-direction: column;
        row-gap: 16px;
    }

    &__search-container {
        display: flex;
        align-items: center;
        background: var(--c-white);
        border: 1px solid var(--c-grey-3);
        border-radius: 8px;
        padding: 12px;
        margin-top: 16px;

        svg {
            min-width: 17px;
            margin-right: 12px;
        }

        input {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);
            border: none;
            outline: none;
        }
    }

    &__selected-container {
        flex-wrap: wrap;
        display: flex;
        align-items: center;
        margin-top: 16px;
        gap: 4px 4px;

        &__item {
            max-width: 100%;
            display: flex;
            align-items: center;
            box-sizing: border-box;
            padding: 6px 16px;
            border: 1px solid var(--c-grey-3);
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 8px;

            &__label {
                margin-right: 8px;
                font-family: 'font_bold', sans-serif;
                font-size: 12px;
                line-height: 20px;
                color: var(--c-black);
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }
        }

        svg {
            cursor: pointer;
            min-width: 14px;
        }
    }

    &__bucket-results {
        padding: 4px 0;
        max-height: 120px;
        overflow-y: auto;
        border: 1px solid var(--c-grey-3);
        box-shadow: 0 4px 6px -2px rgb(0 0 0 / 5%);
        border-radius: 6px;
        max-width: 100%;
        box-sizing: border-box;

        &__item {
            background-color: var(--c-white);
            font-weight: 500;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);
            padding: 10px 16px;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
            text-align: left;
            cursor: pointer;

            &:hover {
                background-color: var(--c-grey-2);
            }
        }

        &__empty {
            padding: 10px 16px;
            font-weight: 500;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);
            text-align: left;
        }
    }

    &__date-selection {
        margin-top: -8px;

        &__label {
            font-size: 14px;
            line-height: 22px;
            text-decoration: underline;
            text-underline-position: under;
            color: var(--c-grey-6);
            text-align: left;
            cursor: pointer;
        }
    }
}
</style>
