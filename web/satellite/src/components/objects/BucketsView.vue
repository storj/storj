// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-view">
        <div class="buckets-view__title-area">
            <h1 class="buckets-view__title-area__title">Buckets</h1>
        </div>
        <div class="buckets-view__loader" v-if="isLoading"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AccessGrant, GatewayCredentials } from '@/types/accessGrants';
import { MetaUtils } from '@/utils/meta';

@Component
export default class BucketsView extends Vue {
    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';
    private worker: Worker;
    private grantWithPermissions: string = '';
    private accessGrant: string = '';

    public isLoading: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Setup gateway credentials.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.passphrase) {
            await this.$router.push(RouteConfig.Objects.path);

            return;
        }

        try {
            const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.FILE_BROWSER_AG_NAME);

            this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
            this.worker.onmessage = (event: MessageEvent) => {
                const data = event.data;
                if (data.error) {
                    throw new Error(data.error);
                }

                this.grantWithPermissions = data.value;
            };

            const now = new Date();
            const inADay = new Date(now.setDate(now.getDate() + 1));

            await this.worker.postMessage({
                'type': 'SetPermission',
                'isDownload': true,
                'isUpload': true,
                'isList': true,
                'isDelete': true,
                'buckets': [],
                'apiKey': cleanAPIKey.secret,
                'notBefore': now.toISOString(),
                'notAfter': inADay.toISOString(),
            });

            // Timeout is used to give some time for web worker to return value.
            setTimeout(() => {
                this.worker.onmessage = (event: MessageEvent) => {
                    const data = event.data;
                    if (data.error) {
                        throw new Error(data.error);
                    }

                    this.accessGrant = data.value;
                };

                const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');
                this.worker.postMessage({
                    'type': 'GenerateAccess',
                    'apiKey': this.grantWithPermissions,
                    'passphrase': this.$route.params.passphrase,
                    'projectID': this.$store.getters.selectedProject.id,
                    'satelliteNodeURL': satelliteNodeURL,
                });

                // Timeout is used to give some time for web worker to return value.
                setTimeout(async () => {
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_ACCESS_GRANT, this.accessGrant);

                    // TODO: use this value until all the satellites will have this URL set.
                    const gatewayURL = 'https://auth.tardigradeshare.io';
                    const gatewayCredentials: GatewayCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: this.accessGrant, optionalURL: gatewayURL});
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
                }, 1000);
            }, 1000);
        } catch (error) {
            await this.$notify.error(`Failed to setup Objects view. ${error.message}`);

            return;
        }
    }

    /**
     * Lifecycle hook before component destroying.
     * Remove temporary created access grant.
     */
    public async beforeDestroy(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }
}
</script>

<style scoped lang="scss">
    .buckets-view {
        display: flex;
        flex-direction: column;
        align-items: center;

        &__title-area {
            margin-bottom: 100px;
            width: 100%;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-style: normal;
                font-weight: bold;
                font-size: 18px;
                line-height: 26px;
                color: #232b34;
                margin: 0;
                width: 100%;
                text-align: left;
            }
        }

        &__loader {
            border: 16px solid #f3f3f3;
            border-top: 16px solid #3498db;
            border-radius: 50%;
            width: 120px;
            height: 120px;
            animation: spin 2s linear infinite;
        }
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
</style>
