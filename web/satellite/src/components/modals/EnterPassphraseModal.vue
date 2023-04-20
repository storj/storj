// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="() => closeModal(true)">
        <template #content>
            <div class="modal">
                <EnterPassphraseIcon />
                <h1 class="modal__title">Enter your encryption passphrase</h1>
                <p class="modal__info">
                    To open a project and view your encrypted files, <br>please enter your encryption passphrase.
                </p>
                <VInput
                    label="Encryption Passphrase"
                    placeholder="Enter your passphrase"
                    :error="enterError"
                    role-description="passphrase"
                    is-password
                    @setData="setPassphrase"
                />
                <div class="modal__buttons">
                    <VButton
                        label="Enter without passphrase"
                        height="48px"
                        font-size="14px"
                        :is-transparent="true"
                        :on-press="() => closeModal()"
                    />
                    <VButton
                        label="Continue ->"
                        height="48px"
                        font-size="14px"
                        :on-press="onContinue"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/router';
import { useRouter } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import EnterPassphraseIcon from '@/../static/images/buckets/openBucket.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const enterError = ref<string>('');
const passphrase = ref<string>('');

/**
 * Sets passphrase.
 */
function onContinue(): void {
    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';
        analytics.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

        return;
    }

    bucketsStore.setPassphrase(passphrase.value);
    bucketsStore.setPromptForPassphrase(false);

    closeModal();
}

/**
 * Closes enter passphrase modal and navigates to single project dashboard from
 * all projects dashboard.
 */
function closeModal(isCloseButton = false): void {
    if (!isCloseButton && router.currentRoute.name === RouteConfig.AllProjectsDashboard.name) {
        router.push(RouteConfig.ProjectDashboard.path);
    }

    appStore.removeActiveModal();
}

/**
 * Sets passphrase from child component.
 */
function setPassphrase(value: string): void {
    if (enterError.value) enterError.value = '';

    passphrase.value = value;
}
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 62px 62px 54px;
        max-width: 500px;

        @media screen and (max-width: 600px) {
            padding: 62px 24px 54px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 26px;
            line-height: 31px;
            color: #131621;
            margin: 30px 0 15px;
        }

        &__info {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin-bottom: 32px;
        }

        &__buttons {
            display: flex;
            column-gap: 20px;
            margin-top: 31px;
            width: 100%;

            @media screen and (max-width: 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 20px;
            }
        }
    }
</style>
