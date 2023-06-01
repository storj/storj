// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="recovery">
                <h1 class="recovery__title">Two-Factor Authentication</h1>
                <div class="recovery__codes">
                    <p class="recovery__codes__subtitle">
                        Please save these codes somewhere to be able to recover access to your account.
                    </p>
                    <p
                        v-for="(code, index) in userMFARecoveryCodes"
                        :key="index"
                    >
                        {{ code }}
                    </p>
                </div>
                <VButton
                    class="recovery__done-button"
                    label="Done"
                    width="100%"
                    height="44px"
                    :on-press="closeModal"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();

/**
 * Returns MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .recovery {
        padding: 60px;
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        @media screen and (width <= 550px) {
            padding: 48px 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            text-align: center;
            color: #000;
            margin: 0 0 30px;

            @media screen and (width <= 550px) {
                font-size: 24px;
                line-height: 28px;
                margin-bottom: 15px;
            }
        }

        &__codes {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            width: calc(100% - 50px);
            display: flex;
            flex-direction: column;
            align-items: center;

            &__subtitle {
                font-size: 16px;
                line-height: 21px;
                text-align: center;
                color: #000;
                margin: 0 0 30px;
                max-width: 485px;

                @media screen and (width <= 550px) {
                    font-size: 14px;
                    line-height: 18px;
                    margin-bottom: 15px;
                }
            }
        }

        &__done-button {
            margin-top: 30px;
        }
    }
</style>
