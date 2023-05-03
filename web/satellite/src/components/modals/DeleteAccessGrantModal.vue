// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <DeleteIcon />
                    <h1 class="modal__header__title">Delete Access</h1>
                </div>
                <p class="modal__info">The following access will be deleted.</p>
                <h3 class="modal__name">{{ name }}</h3>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        :is-white="true"
                        height="52px"
                        width="100%"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Delete"
                        :is-solid-delete="true"
                        icon="trash"
                        width="100%"
                        height="52px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onDelete"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import DeleteIcon from '@/../static/images/modals/deleteAccessGrant/delete.svg';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

/**
 * Returns access name to delete from store.
 */
const name = computed((): string => {
    return agStore.state.accessNameToDelete;
});

/**
 * Closes delete access grant modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Deletes selected access grant.
 */
async function onDelete(): Promise<void> {
    await withLoading(async () => {
        try {
            let page = agStore.state.cursor.page;
            if (agStore.state.page.accessGrants.length === 1) {
                page--;
                if (page < 1) {
                    page = 1;
                }
            }

            await agStore.deleteAccessGrants();
            notify.success(`Access Grant deleted successfully`);

            await agStore.getAccessGrants(page, projectsStore.state.selectedProject.id);
            agStore.clearSelection();
            agStore.setAccessNameToDelete('');

            closeModal();
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.CONFIRM_DELETE_AG_MODAL);
        }
    });
}
</script>

<style scoped lang="scss">
.modal {
    width: 350px;
    padding: 32px;
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;

    @media screen and (max-width: 500px) {
        width: unset;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: var(--c-black);
            margin-left: 16px;
            text-align: left;
        }
    }

    &__info {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        margin-bottom: 24px;
        text-align: left;
    }

    &__name {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        padding-bottom: 16px;
        margin-bottom: 20px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__buttons {
        display: flex;
        align-items: center;
        column-gap: 16px;

        @media screen and (max-width: 500px) {
            column-gap: unset;
            row-gap: 10px;
            flex-direction: column-reverse;
        }
    }
}
</style>
