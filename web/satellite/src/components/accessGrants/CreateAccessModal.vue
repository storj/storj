// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="access-grant">
        <div class="access-grant__modal-container">
            <!-- ********* Create Form Modal ********* -->
            <form v-if="isCreateStep">
                <CreateFormModal
                    :checked-type="checkedType"
                    @close-modal="onCloseClick"
                    @encrypt="encryptStep"
                    @propogateInfo="createAccessGrantHelper"
                    @input="inputHandler"
                />
            </form>
            <!-- *********   Encrypt Form Modal  ********* -->
            <form v-if="isEncryptStep">
                <EncryptFormModal 
                    @close-modal="onCloseClick"
                    @create-access="createAccessGrant"
                    @backAction="backAction"
                />
            </form>
            <!-- *********   Grant Created Modal  ********* -->
            <form v-if="isGrantCreatedStep">
                <GrantCreatedModal
                    :checked-type="checkedType"
                    :restricted-key="restrictedKey"
                    :access="access"
                    :access-name="accessName"
                    @close-modal="onCloseClick"
                />
            </form>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import CreateFormModal from '@/components/accessGrants/modals/CreateFormModal.vue';
import EncryptFormModal from '@/components/accessGrants/modals/EncryptFormModal.vue';
import GrantCreatedModal from '@/components/accessGrants/modals/GrantCreatedModal.vue';

// for future use when notes is implemented
// import NotesIcon from '@/../static/images/accessGrants/create-access_notes.svg';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AccessGrant } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from "@/store/modules/buckets";
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { RouteConfig } from '@/router';


// TODO: a lot of code can be refactored/reused/split into modules
// @vue/component
@Component({
    components: {
        CreateFormModal,
        EncryptFormModal,
        GrantCreatedModal,
        // for future use when notes is implemented
        // NotesIcon,
    },
})
export default class CreateAccessModal extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    @Prop({default: 'Default'})
    private readonly defaultType: string;

    private accessGrantStep = "create";
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
    private passphrase = "";
    private accessName = '';
    public areBucketNamesFetching = true;

    /**
     * Created Access Grant
     */
    private access = "";

    private worker: Worker;
    private restrictedKey = '';
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    /**
     * Sends "trackPageVisit" event to segment and opens link.
     */ 
    public trackPageVisit(link: string): void {
        this.analytics.pageVisit(link);
    }  

    /**
     * Checks which type was selected and retrieves buckets on mount.
     */
    public async mounted(): Promise<void> {
        this.checkedType = this.$route.params.accessType;
        this.setWorker();
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }
    }

    public encryptStep(): void {
        this.accessGrantStep = 'encrypt';
    }

    public inputHandler(e): void {
        this.checkedType = e; 
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
        this.checkedType = await data.checkedType;
        this.accessName = await data.accessName;
        this.selectedPermissions = await data.selectedPermissions;
        if(type === 'api') {
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
        }

        if (this.notBeforePermission) permissionsMsg = Object.assign(permissionsMsg, {'notBefore': this.notBeforePermission.toISOString()});
        if (this.notAfterPermission) permissionsMsg = Object.assign(permissionsMsg, {'notAfter': this.notAfterPermission.toISOString()});

        await this.worker.postMessage(permissionsMsg);

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error)
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


        if (this.checkedType === 's3') {
            try {
                await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: this.access});

                await this.$notify.success('Gateway credentials were generated successfully');
                
                await this.analytics.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);

                this.areKeysVisible = true;
            } catch (error) {
                await this.$notify.error(error.message);
            }
        } else if (this.checkedType === 'api') {
            await this.analytics.eventTriggered(AnalyticsEvent.API_ACCESS_CREATED);
        } else if (this.checkedType === 'access') {
            await this.analytics.eventTriggered(AnalyticsEvent.ACCESS_GRANT_CREATED);
        }

        this.analytics.eventTriggered(AnalyticsEvent.CREATE_KEYS_CLICKED); 
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
    public get isEncryptStep(): boolean {
        return this.accessGrantStep === 'encrypt';
    }
    public get isGrantCreatedStep(): boolean {
        return this.accessGrantStep === 'grantCreated';
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin: 0;
        width: 0;
    }

    p {
        font-weight: bold;
        padding-bottom: 10px;
    }

    form {
        width: 100%;
    }

    .access-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: flex-start;
        justify-content: center;

        & > * {
            font-family: sans-serif;
        }

        &__modal-container {
            background: #fff;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            position: relative;
            padding: 25px 40px;
            margin-top: 40px;
            width: 410px;
            height: auto;
        }
    }

    a {
        color: #fff;
        text-decoration: underline !important;
        cursor: pointer;
    }

    @media screen and (max-width: 500px) {

        .access-grant__modal-container {
            width: auto;
            max-width: 80vw;
            padding: 30px 24px;

            &__body-container {
                grid-template-columns: 1.2fr 6fr;
            }
        }
    }

    @media screen and (max-height: 800px) {

        .access-grant {
            padding: 50px 0 20px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 750px) {

        .access-grant {
            padding: 100px 0 20px;
        }
    }

    @media screen and (max-height: 700px) {

        .access-grant {
            padding: 150px 0 20px;
        }
    }

    @media screen and (max-height: 650px) {

        .access-grant {
            padding: 200px 0 20px;
        }
    }

    @media screen and (max-height: 600px) {

        .access-grant {
            padding: 250px 0 20px;
        }
    }

    @media screen and (max-height: 550px) {

        .access-grant {
            padding: 300px 0 20px;
        }
    }

    @media screen and (max-height: 500px) {

        .access-grant {
            padding: 350px 0 20px;
        }
    }
</style>
