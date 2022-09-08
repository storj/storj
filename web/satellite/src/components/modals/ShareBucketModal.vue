// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Share Bucket</h1>
                <p class="modal__label">
                    Share this link via...
                </p>
                <ShareContainer :link="link" />
                <p class="modal__label">
                    Or copy link
                </p>
                <VLoader v-if="isLoading" width="20px" height="20px" />
                <p v-else class="modal__link">{{ link }}</p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="48px"
                        is-transparent="true"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        :label="copyButtonState === ButtonStates.Copy ? 'Copy Link' : 'Link Copied'"
                        height="48px"
                        :on-press="onCopy"
                        :is-disabled="isLoading"
                        :is-green-white="copyButtonState === ButtonStates.Copied"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { MetaUtils } from '@/utils/meta';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';

import VModal from '@/components/common/VModal.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import ShareContainer from '@/components/common/share/ShareContainer.vue';

enum ButtonStates {
    Copy,
    Copied,
}

// @vue/component
@Component({
    components: {
        VModal,
        VButton,
        VLoader,
        ShareContainer,
    },
})
export default class ShareBucketModal extends Vue {
    private worker: Worker;
    private readonly ButtonStates = ButtonStates;

    public isLoading = true;
    public link = '';
    public copyButtonState = ButtonStates.Copy;

    /**
     * Lifecycle hook after initial render.
     * Sets local worker.
     */
    public async mounted(): Promise<void> {
        this.setWorker();
        await this.setShareLink();
    }

    /**
     * Copies link to users clipboard.
     */
    public async onCopy(): Promise<void> {
        await this.$copyText(this.link);
        this.copyButtonState = ButtonStates.Copied;

        setTimeout(() => {
            this.copyButtonState = ButtonStates.Copy;
        }, 2000);

        await this.$notify.success('Link copied successfully.');
    }

    /**
     * Sets share bucket link.
     */
    private async setShareLink(): Promise<void> {
        try {
            let path = `${this.bucketName}`;
            const now = new Date();
            const LINK_SHARING_AG_NAME = `${path}_shared-bucket_${now.toISOString()}`;
            const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, LINK_SHARING_AG_NAME);

            const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');

            this.worker.postMessage({
                'type': 'GenerateAccess',
                'apiKey': cleanAPIKey.secret,
                'passphrase': this.passphrase,
                'projectID': this.$store.getters.selectedProject.id,
                'satelliteNodeURL': satelliteNodeURL,
            });

            const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
            const grantData = grantEvent.data;
            if (grantData.error) {
                await this.$notify.error(grantData.error);

                return;
            }

            this.worker.postMessage({
                'type': 'RestrictGrant',
                'isDownload': true,
                'isUpload': false,
                'isList': true,
                'isDelete': false,
                'paths': [path],
                'grant': grantData.value,
            });

            const event: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
            const data = event.data;
            if (data.error) {
                await this.$notify.error(data.error);

                return;
            }

            const credentials: EdgeCredentials =
                await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant: data.value, isPublic: true });

            path = encodeURIComponent(path.trim());

            const linksharingURL = MetaUtils.getMetaContent('linksharing-url');

            this.link = `${linksharingURL}/${credentials.accessKeyId}/${path}`;
        } catch (error) {
            await this.$notify.error(error.message);
        } finally {
            this.isLoading = false;
        }
    }

    /**
     * Sets local worker with worker instantiated in store.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Closes open bucket modal.
     */
    public closeModal(): void {
        if (this.isLoading) return;

        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_SHARE_BUCKET_MODAL_SHOWN);
    }

    /**
     * Returns chosen bucket name from store.
     */
    private get bucketName(): string {
        return this.$store.state.objectsModule.fileComponentBucketName;
    }

    /**
     * Returns passphrase from store.
     */
    private get passphrase(): string {
        return this.$store.state.objectsModule.passphrase;
    }
}
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 35px 35px 35px 50px;
        max-width: 470px;

        @media screen and (max-width: 430px) {
            padding: 20px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 22px;
            line-height: 29px;
            color: #1b2533;
        }

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            align-self: flex-start;
            margin: 32px 0 16px;
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
    }
</style>
