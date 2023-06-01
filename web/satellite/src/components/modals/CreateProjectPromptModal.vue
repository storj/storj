// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <ProjectIcon />
                    <h1 class="modal__header__title">Get more projects</h1>
                </div>
                <p v-if="!user.paidTier" class="modal__info">
                    Upgrade to Pro Account to create more projects and gain access to higher limits.
                </p>
                <p v-else class="modal__info">
                    Request project limits increase.
                </p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        :on-press="closeModal"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        is-white
                    />
                    <VButton
                        :label="buttonLabel()"
                        :on-press="onClick"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import ProjectIcon from '@/../static/images/common/blueBox.svg';

const appStore = useAppStore();
const userStore = useUsersStore();
const user = userStore.state.user;

/**
 * Button text logic depending on if the user is in the free or paid tier.
 */
function buttonLabel(): string {
    let label = 'Upgrade -->';

    if (user.paidTier) {
        label = 'Request -->';
    }

    return label;
}

/**
 * Holds on button click logic.
 * Closes this modal
 * Redirects to upgrade modal or opens new tab to request increase project limits .
 */
function onClick(): void {
    if (!user.paidTier) {
        appStore.updateActiveModal(MODALS.upgradeAccount);
    } else {
        appStore.removeActiveModal();
        window.open('https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212', '_blank', 'noopener');
    }
}

/**
 * Closes create project prompt modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
.modal {
    width: 414px;
    padding: 32px;
    font-family: 'font_regular', sans-serif;

    @media screen and (max-width <= 375px) {
        width: unset;
        padding: 32px 16px;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: 800;
            font-size: 24px;
            line-height: 31px;
            color: var(--c-black);
            margin-left: 16px;
        }
    }

    &__info {
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        margin-top: 16px;
        text-align: left;
    }

    &__buttons {
        border-top: 1px solid var(--c-grey-2);
        margin-top: 16px;
        padding-top: 24px;
        display: flex;
        align-items: center;
        column-gap: 16px;
    }
}
</style>
