// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Share File</h1>
                <ShareContainer :link="link" />
                <p class="modal__label">
                    Or copy link:
                </p>
                <VLoader v-if="isLoading" width="20px" height="20px" />
                <div v-if="!isLoading" class="modal__input-group">
                    <input
                        id="url"
                        class="modal__input"
                        type="url"
                        :value="link"
                        aria-describedby="btn-copy-link"
                        readonly
                    >
                    <VButton
                        :label="copyButtonState === ButtonStates.Copy ? 'Copy' : 'Copied'"
                        width="114px"
                        height="40px"
                        :on-press="onCopy"
                        :is-disabled="isLoading"
                        :is-green="copyButtonState === ButtonStates.Copied"
                        :icon="copyButtonState === ButtonStates.Copied ? 'none' : 'copy'"
                    >
                        <template v-if="copyButtonState === ButtonStates.Copied" #icon>
                            <check-icon />
                        </template>
                    </VButton>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useStore } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import ShareContainer from '@/components/common/share/ShareContainer.vue';

import CheckIcon from '@/../static/images/common/check.svg';

enum ButtonStates {
    Copy,
    Copied,
}

const appStore = useAppStore();
const store = useStore();
const notify = useNotify();

const isLoading = ref<boolean>(true);
const link = ref<string>('');
const copyButtonState = ref<ButtonStates>(ButtonStates.Copy);

/**
 * Retrieve the path to the current file that has the fileShareModal opened from the store.
 */
const filePath = computed((): string => {
    return store.state.files.objectPathForModal;
});

/**
 * Copies link to users clipboard.
 */
async function onCopy(): Promise<void> {
    await navigator.clipboard.writeText(link.value);
    copyButtonState.value = ButtonStates.Copied;

    setTimeout(() => {
        copyButtonState.value = ButtonStates.Copy;
    }, 2000);

    await notify.success('Link copied successfully.');
}

/**
 * Closes open bucket modal.
 */
function closeModal(): void {
    if (isLoading.value) return;

    appStore.updateActiveModal(MODALS.shareObject);
}

/**
 * Lifecycle hook after initial render.
 * Sets share link.
 */
onMounted(async (): Promise<void> => {
    link.value = await store.state.files.fetchSharedLink(
        filePath.value,
    );

    isLoading.value = false;
});
</script>

<style scoped lang="scss">
.modal {
    font-family: 'font_regular', sans-serif;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 50px;
    max-width: 470px;

    @media screen and (max-width: 430px) {
        padding: 20px;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 22px;
        line-height: 29px;
        color: #1b2533;
        margin: 10px 0 35px;
    }

    &__label {
        font-family: 'font_medium', sans-serif;
        font-size: 14px;
        line-height: 21px;
        color: #354049;
        align-self: center;
        margin: 20px 0 10px;
    }

    &__link {
        font-size: 16px;
        line-height: 21px;
        color: #384b65;
        align-self: flex-start;
        word-break: break-all;
        text-align: left;
    }

    &__buttons {
        display: flex;
        column-gap: 20px;
        margin-top: 32px;
        width: 100%;

        @media screen and (max-width: 430px) {
            flex-direction: column-reverse;
            column-gap: unset;
            row-gap: 15px;
        }
    }

    &__input-group {
        border: 1px solid var(--c-grey-4);
        background: var(--c-grey-1);
        padding: 10px;
        display: flex;
        justify-content: space-between;
        border-radius: 8px;
        width: 100%;
        height: 42px;
    }

    &__input {
        background: none;
        border: none;
        font-size: 14px;
        color: var(--c-grey-6);
        outline: none;
        max-width: 340px;
        width: 100%;

        @media screen and (max-width: 430px) {
            max-width: 210px;
        }
    }
}
</style>
