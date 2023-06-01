// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
    >
        <template #options>
            <th v-click-outside="closeDropdown" class="grant-item__functional options overflow-visible" @click.stop="openDropdown">
                <dots-icon />
                <div v-if="isDropdownOpen" class="grant-item__functional__dropdown">
                    <div class="grant-item__functional__dropdown__item" @click.stop="onDeleteClick">
                        <delete-icon />
                        <p class="grant-item__functional__dropdown__item__label">Delete Access</p>
                    </div>
                </div>
            </th>
        </template>
    </table-item>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import DeleteIcon from '../../../static/images/objects/delete.svg';
import DotsIcon from '../../../static/images/objects/dots.svg';

import { AccessGrant } from '@/types/accessGrants';
import { useResize } from '@/composables/resize';

import TableItem from '@/components/common/TableItem.vue';

const props = withDefaults(defineProps<{
    itemData: AccessGrant,
    onClick?: () => void,
    isDropdownOpen: boolean,
    dropdownKey: number,
}>(), {
    itemData: () => new AccessGrant('', '', new Date(), ''),
    onClick: () => {},
    isDropdownOpen: false,
    dropdownKey: -1,
});

const emit = defineEmits(['openDropdown', 'deleteClick']);

const { isMobile } = useResize();

const itemToRender = computed((): { [key: string]: string | string[] } => {
    if (!isMobile.value) return { name: props.itemData.name, date: props.itemData.localDate() };

    return { info: [ props.itemData.name, `Created ${props.itemData.localDate()}` ] };
});

/**
 * Closes dropdown.
 */
function closeDropdown(): void {
    emit('openDropdown', -1);
}

/**
 * Opens dropdown.
 */
function openDropdown(): void {
    emit('openDropdown', props.dropdownKey);
}

async function onDeleteClick(): Promise<void> {
    emit('deleteClick', props.itemData);
    closeDropdown();
}
</script>

<style scoped lang="scss">
    .grant-item {

        &__functional {
            padding: 0 10px;
            position: relative;
            cursor: pointer;

            &__dropdown {
                position: absolute;
                top: 55px;
                right: 15px;
                background: #fff;
                box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
                border-radius: 6px;
                width: 255px;
                z-index: 100;
                overflow: hidden;

                &__item {
                    display: flex;
                    align-items: center;
                    padding: 20px 25px;
                    width: calc(100% - 50px);

                    &__label {
                        margin: 0 0 0 10px;
                    }

                    &:hover {
                        background-color: #f4f5f7;
                        font-family: 'font_medium', sans-serif;
                        color: var(--c-blue-3);

                        svg :deep(path) {
                            fill: var(--c-blue-3);
                        }
                    }
                }
            }
        }
    }

    :deep(.primary) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    :deep(th) {
        max-width: 25rem;
    }

    @media screen and (width <= 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>
