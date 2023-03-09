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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { OBJECTS_MUTATIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/router';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import EnterPassphraseIcon from '@/../static/images/buckets/openBucket.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VModal,
        VButton,
        EnterPassphraseIcon,
    },
})
export default class EnterPassphraseModal extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public enterError = '';
    public passphrase = '';

    /**
     * Sets passphrase.
     */
    public onContinue(): void {
        if (!this.passphrase) {
            this.enterError = 'Passphrase can\'t be empty';
            this.analytics.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

            return;
        }

        this.$store.commit(OBJECTS_MUTATIONS.SET_PASSPHRASE, this.passphrase);
        this.$store.commit(OBJECTS_MUTATIONS.SET_PROMPT_FOR_PASSPHRASE, false);

        this.closeModal();
    }

    /**
     * Closes enter passphrase modal and navigates to single project dashboard from
     * all projects dashboard.
     */
    public closeModal(isCloseButton = false): void {
        if (!isCloseButton && this.$route.name === RouteConfig.AllProjectsDashboard.name) {
            this.$router.push(RouteConfig.ProjectDashboard.path);
        }

        this.$store.commit(APP_STATE_MUTATIONS.REMOVE_ACTIVE_MODAL);
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = passphrase;
    }
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
