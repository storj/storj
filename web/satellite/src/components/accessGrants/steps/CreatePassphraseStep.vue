// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-passphrase">
        <BackIcon class="create-passphrase__back-icon" @click="onBackClick"/>
        <GeneratePassphrase
            :is-loading="isLoading"
            :on-button-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        BackIcon,
        GeneratePassphrase,
    },
})
export default class CreatePassphraseStep extends Vue {
    private key: string = '';
    private restrictedKey: string = '';
    private access: string = '';
    private worker: Worker;
    private isLoading: boolean = true;

    public passphrase: string = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            await this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;

        this.setWorker();

        this.isLoading = false;
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onmessage = (event: MessageEvent) => {
            const data = event.data;
            if (data.error) {
                this.$notify.error(data.error);

                return;
            }

            this.access = data.value;

            this.$notify.success('Access Grant was generated successfully');
        };
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
    }

    /**
     * Holds on next button click logic.
     * Generates access grant and redirects to next step.
     */
    public onNextClick(): void {
        if (this.isLoading) return;

        this.isLoading = true;

        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': this.restrictedKey,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        // Give time for web worker to return value.
        setTimeout(() => {
            this.isLoading = false;

            if (this.isOnboardingTour) {
                this.$router.push({
                    name: RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant.with(RouteConfig.AccessGrantResult)).name,
                    params: {
                        access: this.access,
                        key: this.key,
                        restrictedKey: this.restrictedKey,
                    },
                });

                return;
            }

            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).name,
                params: {
                    access: this.access,
                    key: this.key,
                    restrictedKey: this.restrictedKey,
                },
            });
        }, 1000);
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        if (this.isOnboardingTour) {
            this.$router.push({
                name: RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant.with(RouteConfig.AccessGrantPermissions)).name,
                params: {
                    key: this.key,
                },
            });

            return;
        }

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    private get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }
}
</script>

<style scoped lang="scss">
    .create-passphrase {
        position: relative;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }
    }
</style>
