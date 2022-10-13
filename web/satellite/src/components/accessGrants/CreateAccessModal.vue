// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onCloseClick">
        <template #content>
            <CreateForm
                v-if="isCreateStep"
                :checked-type="checkedType"
                @encrypt="encryptStep"
                @propagateInfo="createAccessGrantHelper"
            />
            <EncryptionInfoForm
                v-if="isEncryptInfoStep"
                @back="onBackFromEncryptionInfo"
                @continue="onContinueFromEncryptionInfo"
            />
            <EncryptForm
                v-if="isEncryptStep"
                @apply-passphrase="applyPassphrase"
                @create-access="createAccessGrant"
                @backAction="backAction"
            />
            <GrantCreatedForm
                v-if="isGrantCreatedStep"
                :checked-type="checkedType"
                :restricted-key="restrictedKey"
                :access="access"
                :access-name="accessName"
            />
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AccessGrant } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { RouteConfig } from '@/router';
import { LocalData } from '@/utils/localData';

import VModal from '@/components/common/VModal.vue';
import EncryptionInfoForm from '@/components/accessGrants/modals/EncryptionInfoForm.vue';
import GrantCreatedForm from '@/components/accessGrants/modals/GrantCreatedForm.vue';
import EncryptForm from '@/components/accessGrants/modals/EncryptForm.vue';
import CreateForm from '@/components/accessGrants/modals/CreateForm.vue';

// TODO: a lot of code can be refactored/reused/split into modules
// @vue/component
@Component({
    components: {
        VModal,
        CreateForm,
        EncryptForm,
        GrantCreatedForm,
        EncryptionInfoForm,
    },
})
export default class CreateAccessModal extends Vue {
    @Prop({ default: 'Default' })
    private readonly label: string;
    @Prop({ default: 'Default' })
    private readonly defaultType: string;

    private accessGrantStep = 'create';
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
    public areKeysVisible = false;
    private readonly FIRST_PAGE = 1;

    /**
     * Stores access type that is selected and text changes based on type.
     */
    private checkedType = '';

    /**
     * Global isLoading Variable
     **/
    private isLoading = false;

    /**
     * Handles permission types, which have been selected, and determining if all have been selected.
     */
    private selectedPermissions : string[] = [];

    /**
     * Handles business logic for options on each step after create access.
     */
    private passphrase = '';
    private accessName = '';
    public areBucketNamesFetching = true;

    /**
     * Created Access Grant
     */
    private access = '';

    private worker: Worker;
    private restrictedKey = '';
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    public beforeMount(): void {
        this.checkedType = this.$route.params.accessType;
    }

    /**
     * Checks which type was selected and retrieves buckets on mount.
     */
    public async mounted(): Promise<void> {
        this.setWorker();
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }
    }

    public encryptStep(): void {
        if (!LocalData.getServerSideEncryptionModalHidden() && this.checkedType.includes('s3')) {
            this.accessGrantStep = 'encryptInfo';
            return;
        }

        this.accessGrantStep = 'encrypt';
    }

    public applyPassphrase(passphrase: string) {
        this.passphrase = passphrase;
    }

    public onBackFromEncryptionInfo() {
        this.accessGrantStep = 'create';
    }

    public onContinueFromEncryptionInfo() {
        this.accessGrantStep = 'encrypt';
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Grabs data from child for createAccessGrant
     */
    public async createAccessGrantHelper(data, type): Promise<void> {
        this.checkedType = data.checkedType;
        this.accessName = data.accessName;
        this.selectedPermissions = data.selectedPermissions;

        if (type === 'api') {
            await this.createAccessGrant();
        }
    }

    /**
     * Creates Access Grant
     */
    public async createAccessGrant(): Promise<void> {
        if (this.$store.getters.projects.length === 0) {
            try {
                await this.$store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT);
            } catch (error) {
                this.isLoading = false;
                return;
            }
        }

        // creates restricted key
        let cleanAPIKey: AccessGrant;
        try {
            cleanAPIKey = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.accessName);
        } catch (error) {
            await this.$notify.error(error.message);
            return;
        }

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);

            this.isLoading = false;
        }

        let permissionsMsg = {
            'type': 'SetPermission',
            'buckets': this.selectedBucketNames,
            'apiKey': cleanAPIKey.secret,
            'isDownload': this.selectedPermissions.includes('Read'),
            'isUpload': this.selectedPermissions.includes('Write'),
            'isList': this.selectedPermissions.includes('List'),
            'isDelete': this.selectedPermissions.includes('Delete'),
        };

        if (this.notBeforePermission) permissionsMsg = Object.assign(permissionsMsg, { 'notBefore': this.notBeforePermission.toISOString() });
        if (this.notAfterPermission) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': this.notAfterPermission.toISOString() });

        await this.worker.postMessage(permissionsMsg);

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }
        this.restrictedKey = grantEvent.data.value;

        // creates access credentials
        const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': this.restrictedKey,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (accessEvent.data.error) {
            await this.$notify.error(accessEvent.data.error);
            this.isLoading = false;
            return;
        }

        this.access = accessEvent.data.value;
        await this.$notify.success('Access Grant was generated successfully');

        if (this.checkedType === 's3' || (this.checkedType.includes('s3') && this.checkedType.includes('access'))) {
            try {
                await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant: this.access });

                await this.analytics.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);

                await this.$notify.success('Gateway credentials were generated successfully');

                this.areKeysVisible = true;
            } catch (error) {
                await this.$notify.error(error.message);
            }
        } else {
            await this.analytics.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED);
        }

        this.analytics.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED);
        this.accessGrantStep = 'grantCreated';
    }

    public backAction(): void {
        this.accessGrantStep = 'create';
        this.passphrase = '';
    }

    /**
     * Closes modal.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Retrieves selected buckets for bucket bullets.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }

    /**
     * Returns not before date permission from store.
     */
    private get notBeforePermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotBefore;
    }

    /**
     * Returns not after date permission from store.
     */
    private get notAfterPermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotAfter;
    }

    /**
     * Returns which step should be rendered.
     */
    public get isCreateStep(): boolean {
        return this.accessGrantStep === 'create';
    }
    public get isEncryptInfoStep(): boolean {
        return this.accessGrantStep === 'encryptInfo';
    }
    public get isEncryptStep(): boolean {
        return this.accessGrantStep === 'encrypt';
    }
    public get isGrantCreatedStep(): boolean {
        return this.accessGrantStep === 'grantCreated';
    }
}
</script>
