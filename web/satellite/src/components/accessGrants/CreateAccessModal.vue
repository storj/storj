// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="access-grant">
        <div class="access-grant__modal-container">
            <div 
                v-if="tooltipHover === 'access'" 
                class="access-tooltip"
                @mouseover="toggleTooltipHover('access','over')"
                @mouseleave="toggleTooltipHover('access','leave')"
            >
                <span class="tooltip-text">Keys to upload, delete, and view your project's data.  <a class="tooltip-link" href="https://storj-labs.gitbook.io/dcs/concepts/access/access-grants" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <div 
                v-if="tooltipHover === 's3'" 
                class="s3-tooltip"
                @mouseover="toggleTooltipHover('s3','over')"
                @mouseleave="toggleTooltipHover('s3','leave')"
            >
                <span class="tooltip-text">Generates access key, secret key, and endpoint to use in your S3-supporting application.  <a class="tooltip-link" href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <div 
                v-if="tooltipHover === 'api'" 
                class="api-tooltip"
                @mouseover="toggleTooltipHover('api','over')"
                @mouseleave="toggleTooltipHover('api','leave')"
            >
                <span class="tooltip-text">Creates access grant to run in the command line.  <a class="tooltip-link" href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token/" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <!-- ********* Create Form Modal ********* -->
            <form v-if="accessGrantStep === 'create'">
                <div class="access-grant__modal-container__header-container">
                    <h2 class="access-grant__modal-container__header-container__title">Create Access</h2>
                    <div
                        class="access-grant__modal-container__header-container__close-cross-container" @click="onCloseClick"
                    >
                        <CloseCrossIcon />
                    </div>
                </div>
                <div class="access-grant__modal-container__body-container">
                    <TypesIcon class="access-grant__modal-container__body-container__type-icon" />
                    <div class="access-grant__modal-container__body-container__type">
                        <p>Type</p>
                        <div class="access-grant__modal-container__body-container__type__type-container">
                            <input 
                                id="acess-grant-check"
                                v-model="checkedType"
                                value="access" 
                                type="radio"
                                name="type" 
                                :checked="checkedType === 'access'"
                            >
                            <label for="acess-grant-check">
                                Access Grant
                            </label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('access','over')"
                                @mouseleave="toggleTooltipHover('access','leave')"
                            >
                        </div>
                        <div class="access-grant__modal-container__body-container__type__type-container">
                            <input 
                                id="s3-check"
                                v-model="checkedType"
                                value="s3" 
                                type="radio"
                                name="type" 
                                :checked="checkedType === 's3'"
                            >
                            <label for="s3-check">S3 Credentials</label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('s3','over')"
                                @mouseleave="toggleTooltipHover('s3','leave')"
                            >
                        </div>
                        <div class="access-grant__modal-container__body-container__type__type-container">
                            <input
                                id="api-check"
                                v-model="checkedType"
                                value="api" 
                                type="radio"
                                name="type" 
                                :checked="checkedType === 'api'"
                                @click="checkedType = 'api'"
                            >
                            <label for="api-check">API Access</label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('api','over')"
                                @mouseleave="toggleTooltipHover('api','leave')"
                            >
                        </div>
                    </div>
                    <NameIcon class="access-grant__modal-container__body-container__name-icon" />
                    <div class="access-grant__modal-container__body-container__name">
                        <p>Name</p>
                        <input
                            v-model="accessName"
                            type="text" 
                            placeholder="Input Access Name" class="access-grant__modal-container__body-container__name__input"
                        >
                    </div>
                    <PermissionsIcon class="access-grant__modal-container__body-container__permissions-icon" />
                    <div class="access-grant__modal-container__body-container__permissions">
                        <p>Permissions</p>
                        <div>
                            <input
                                id="permissions__all-check" 
                                type="checkbox"
                                :checked="allPermissionsClicked"
                                @click="toggleAllPermission('all')"
                            >
                            <label for="permissions__all-check">All</label>
                            <Chevron :class="`permissions-chevron-${showAllPermissions.position}`" @click="togglePermissions" />
                        </div>
                        <div v-if="showAllPermissions.show">
                            <div v-for="(item, key) in permissionsList" :key="key">
                                <input 
                                    :id="`permissions__${item}-check`"
                                    v-model="selectedPermissions"
                                    :value="item" 
                                    type="checkbox"
                                    :checked="checkedPermissions.item"
                                    @click="toggleAllPermission(item)"
                                >
                                <label :for="`permissions__${item}-check`">{{ item }}</label>
                            </div>
                        </div>
                    </div>
                    <BucketsIcon class="access-grant__modal-container__body-container__buckets-icon" />
                    <div class="access-grant__modal-container__body-container__buckets">
                        <p>Buckets</p>
                        <div>
                            <BucketsSelection 
                                class="access-bucket-container"
                                :show-scrollbar="true"
                            />
                        </div>
                        <div class="access-grant__modal-container__body-container__buckets__bucket-bullets">
                            <div
                                v-for="(name, index) in selectedBucketNames"
                                :key="index"
                                class="access-grant__modal-container__body-container__buckets__bucket-bullets__container"
                            >
                                <BucketNameBullet :name="name" />
                            </div>
                        </div>
                    </div>
                    <DateIcon class="access-grant__modal-container__body-container__date-icon" />
                    <div class="access-grant__modal-container__body-container__duration">
                        <p>Duration</p>
                        <div v-if="addDateSelected">
                            <DurationSelection
                                container-style="access-date-container"
                                text-style="access-date-text"
                            />
                        </div>
                        <div 
                            v-else 
                            class="access-grant__modal-container__body-container__duration__text"
                            @click="addDateSelected = true"
                        >
                            Add Date (optional)
                        </div>
                    </div>

                    <!-- for future use when notes feature is implemented -->
                    <!-- <NotesIcon class="access-grant__modal-container__body-container__notes-icon"/>
                    <div class="access-grant__modal-container__body-container__notes">
                        <p>Notes</p>
                        <div>--Notes Section Here--</div>
                    </div> -->
                </div>
                <div class="access-grant__modal-container__divider" />
                <div class="access-grant__modal-container__footer-container">
                    <v-button
                        label="Learn More"
                        width="auto"
                        height="50px"
                        is-transparent="true"
                        font-size="16px"
                        class="access-grant__modal-container__footer-container__learn-more-button"
                    />
                    <v-button
                        label="Encrypt My Access  ⟶"
                        font-size="16px"
                        width="auto"
                        height="50px"
                        class="access-grant__modal-container__footer-container__encrypt-button"
                        :on-press="encryptClickAction"
                        :is-disabled="selectedPermissions.length === 0 || accessName === '' || selectedBucketNames.length === 0"
                    />
                </div>
            </form>
            <!-- *********   Encrypt Form Modal  ********* -->
            <form v-if="accessGrantStep === 'encrypt'">
                <div class="access-grant__modal-container__header-container">
                    <h2 class="access-grant__modal-container__header-container__title">Select Encryption</h2>
                    <div
                        class="access-grant__modal-container__header-container__close-cross-container" @click="onCloseClick"
                    >
                        <CloseCrossIcon />
                    </div>
                </div>
                <div class="access-grant__modal-container__body-container-encrypt"> 
                    <div class="access-grant__modal-container__body-container__encrypt">
                        <AccessKeyIcon class="access-grant__modal-container__body-container__type-icon" />
                        <div class="access-grant__modal-container__body-container__subtext">
                            <p>Generate Passphrase</p>
                            <div>Automatically Generate Seed</div>
                        </div>
                        <div>
                            <input 
                                id="generate-check"
                                v-model="encryptSelect"
                                value="generate" 
                                type="radio"
                                name="type"
                                @change="onRadioInput"
                            >
                        </div>  
                        
                        <ThumbPrintIcon class="access-grant__modal-container__body-container__thumb-icon" />
                        <div class="access-grant__modal-container__body-container__subtext-thumb">
                            <p>Create My Own Passphrase</p>
                            <div>Make it Personalized</div>
                        </div>
                        <div>
                            <input 
                                id="create-check"
                                v-model="encryptSelect"
                                value="create" 
                                type="radio"
                                name="type"
                                @change="onRadioInput"
                            >
                        </div>
                    </div>
                    <div
                        v-if="encryptSelect === 'generate'"
                        class="access-grant__modal-container__generated-passphrase"
                    > 
                        {{ passphrase }}
                    </div>
                    <!-- Working Here -->
                    <input
                        v-if="encryptSelect === 'create'"
                        v-model="passphrase"
                        type="text" 
                        placeholder="Input Your Passphrase" class="access-grant__modal-container__body-container__passphrase"
                        :disabled="encryptSelect === 'generate'"
                    >
                    <div class="access-grant__modal-container__footer-container">
                        <v-button
                            :label="isPassphraseCopied ? 'Copied' : 'Copy to clipboard'"
                            width="auto"
                            height="50px"
                            :is-transparent="isPassphraseCopied ? false : true"
                            :is-white-green="isPassphraseCopied ? true : false"
                            class="access-grant__modal-container__footer-container__copy-button"
                            font-size="16px"
                            :on-press="onCopyClick"
                            :is-disabled="passphrase.length < 1"   
                        />
                        <v-button
                            label="Download .txt"
                            font-size="16px"
                            width="auto"
                            height="50px"
                            class="access-grant__modal-container__footer-container__download-button"
                            :is-green-white="isPassphraseDownloaded ? true : false"
                            :on-press="downloadText"
                            :is-disabled="passphrase.length < 1"
                        />
                    </div>
                    <div v-if="isPassphraseDownloaded || isPassphraseCopied" :class="`access-grant__modal-container__acknowledgement-container ${acknowledgementCheck ? 'blue-background' : ''}`">
                        <input
                            v-model="acknowledgementCheck"
                            type="checkbox"
                            class="access-grant__modal-container__acknowledgement-container__check"
                        >
                        <div class="access-grant__modal-container__acknowledgement-container__text">I understand that Storj does not know or store my encryption passphrase. If I lose it, I won't be able to recover files.</div>
                    </div>
                    <div
                        v-if="isPassphraseDownloaded || isPassphraseCopied"
                        class="access-grant__modal-container__acknowledgement-buttons"
                    >
                        <v-button
                            label="Back"
                            width="auto"
                            height="50px"
                            :is-transparent="true"
                            class="access-grant__modal-container__footer-container__copy-button"
                            font-size="16px"
                            :on-press="backAction"
                        />
                        <v-button
                            label="Create my Access ⟶"
                            font-size="16px"
                            width="auto"
                            height="50px"
                            class="access-grant__modal-container__footer-container__download-button"
                            :is-disabled="!acknowledgementCheck"
                        />
                    </div>
                </div>
            </form>
        </div>
    </div>
</template>


<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import VButton from '@/components/common/VButton.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import BucketNameBullet from "@/components/accessGrants/permissions/BucketNameBullet.vue";
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import TypesIcon from '@/../static/images/accessGrants/create-access_type.svg';
import AccessKeyIcon from '@/../static/images/accessGrants/accessKeyIcon.svg';
import ThumbPrintIcon from '@/../static/images/accessGrants/thumbPrintIcon.svg';
import PermissionsIcon from '@/../static/images/accessGrants/create-access_permissions.svg';
import NameIcon from '@/../static/images/accessGrants/create-access_name.svg';
import BucketsIcon from '@/../static/images/accessGrants/create-access_buckets.svg';
import DateIcon from '@/../static/images/accessGrants/create-access_date.svg';
// for future use when notes is implemented
// import NotesIcon from '@/../static/images/accessGrants/create-access_notes.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';

import { generateMnemonic } from "bip39";
import { AccessGrant } from '@/types/accessGrants';
import { Download } from "@/utils/download";
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from "@/store/modules/buckets";

// @vue/component
@Component({
    components: {
        VButton,
        AccessKeyIcon,
        ThumbPrintIcon,
        DurationSelection,
        BucketsSelection,
        BucketNameBullet,
        CloseCrossIcon,
        TypesIcon,
        PermissionsIcon,
        NameIcon,
        BucketsIcon,
        DateIcon,
        // for future use when notes is implemented
        // NotesIcon,
        Chevron,
    },
})
export default class CreateAccessModal extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    @Prop({default: 'Default'})
    private readonly defaultType: string;

    private accessGrantList = this.accessGrantsList;
    private accessGrantStep = "create";

    /**
     * Stores access type that is selected.
     */
    private checkedType = '';

    /**
     * Handles which tooltip is hovered over and set/clear timeout when leaving hover.
     */
    public tooltipHover = '';
    public tooltipVisibilityTimer;

    /**
     * Handles permission types, which have been selected, and determining if all have been selected.
     */
    private showAllPermissions = {show: false, position: "up"};
    private permissionsList = ["read","write","list","delete"];
    private checkedPermissions = {read: false, write: false, list: false, delete: false};
    private selectedPermissions : string[] = [];
    private allPermissionsClicked = false;
    private acknowledgementCheck = false;

    /**
     * Handles business logic for options on each step after create access.
     */
    private encryptSelect = "create";
    private passphrase = "";
    private isPassphraseCopied = false;
    private isPassphraseDownloaded = false;
    public isGenerateState = false;

    private accessName = '';
    public areBucketNamesFetching = true;
    private addDateSelected = false;


    /**
     * Checks which type was selected and retrieves buckets on mount.
     */
    public async mounted(): Promise<void> {
        this.checkedType = this.defaultType;
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }
    }

    /**
     * Downloads passphrase to .txt file
     */
    public downloadText(): void {
        this.isPassphraseDownloaded = true;
        Download.file(this.passphrase, 'sampleText.txt')
    }

    public onRadioInput(): void {
        this.isPassphraseCopied = false;
        this.isPassphraseDownloaded = false;
        if (this.encryptSelect === "generate") {
            this.passphrase = generateMnemonic();
        }
        else {
            this.passphrase = "";
        }
    }

    public encryptClickAction(): void {
        let mappedList = this.accessGrantList.map((key) => (key.name))
        if (mappedList.includes(this.accessName)) {
            this.$notify.error(`validation: An API Key with this name already exists in this project, please use a different name`);
            return
        } else if (this.checkedType !== "api") {
            this.accessGrantStep = 'encrypt';
        }  
    }

    public onCopyClick(): void {
        this.$copyText(this.passphrase);
        this.isPassphraseCopied = true;
        this.$notify.success('Passphrase was copied successfully');
    }

    public backAction(): void {
        this.accessGrantStep = 'create'
    }

    /**
     * Closes modal.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.$emit('close-modal')
    }

    /**
     * Toggles tooltip visibility.
     */
    public toggleTooltipHover(type,action): void {
        if (this.tooltipHover === '' && action === 'over') {
            this.tooltipHover = type;
            return;
        } else if (this.tooltipHover === type && action === 'leave') {
            this.tooltipVisibilityTimer = setTimeout(() => {
                this.tooltipHover = '';
            },750);
            return;
        } else if (this.tooltipHover === type && action === 'over') {
            clearTimeout(this.tooltipVisibilityTimer);
            return;
        } else if(this.tooltipHover !== type) {
            clearTimeout(this.tooltipVisibilityTimer)
            this.tooltipHover = type;
        }
    }

    /**
     * Toggles permissions list visibility.
     */
    public togglePermissions(): void {
        if (this.showAllPermissions.show === false) {
            this.showAllPermissions.show = true;
            this.showAllPermissions.position = "down";
        } else {
            this.showAllPermissions.show = false;
            this.showAllPermissions.position = "up";
        }
    }

    /**
     * Handles permissions All.
     */
    public toggleAllPermission(type): void {
        if (type === 'all' && this.allPermissionsClicked === false) {
            this.allPermissionsClicked = true;
            this.selectedPermissions = this.permissionsList;
            this.checkedPermissions = {read: true, write: true, list: true, delete: true}
            return
        } else if(type === 'all' && this.allPermissionsClicked === true) {
            this.allPermissionsClicked = false;
            this.selectedPermissions = []
            this.checkedPermissions = {read: false, write: false, list: false, delete: false}
            return
        } else if(this.checkedPermissions[type] === true) {
            this.checkedPermissions[type] = false
            this.allPermissionsClicked = false
            return
        } else {
            this.checkedPermissions[type] = true
            if(this.checkedPermissions.read === true && this.checkedPermissions.write === true && this.checkedPermissions.list === true && this.checkedPermissions.delete === true) {
                this.allPermissionsClicked = true
                return
            }
        }
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
     * Access Grant List
     */
    public get accessGrantsList(): AccessGrant[] {
        return this.$store.state.accessGrantsModule.page.accessGrants;
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

    @mixin chevron {
        padding-left: 4px;
        transition: transform 0.3s;
    }

    @mixin tooltip-container {
        position: absolute;
        background: #56606d;
        border-radius: 6px;
        width: 253px;
        color: #fff;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        padding: 8px;
        z-index: 1;
        transition: 250ms;
    }

    @mixin tooltip-arrow {
        content: '';
        position: absolute;
        bottom: 0;
        width: 0;
        height: 0;
        border: 6px solid transparent;
        border-top-color: #56606d;
        border-bottom: 0;
        margin-left: -20px;
        margin-bottom: -20px;
    }

    p {
        font-weight: bold;
        padding-bottom: 5px;
    }

    label {
        margin-left: 5px;
        padding-right: 10px;
    }

    h2 {
        font-weight: 800;
        font-size: 28px;
    }

    form {
        width: 100%;
    }

    .blue-background {
        background: #d7e8ff;
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
            padding: 25px;
            margin-top: 40px;
            width: 410px;
            height: auto;

            &__generated-passphrase {
                margin-top: 20px;
                align-items: center;
                padding: 10px 16px;
                background: #ebeef1;
                border: 1px solid #c8d3de;
                border-radius: 7px;
            }

            &__header-container {
                text-align: left;
                display: grid;
                grid-template-columns: 2fr 1fr;
                width: 100%;
                padding-top: 10px;

                &__title {
                    grid-column: 1;
                }

                &__close-cross-container {
                    grid-column: 2;
                    margin: auto 0 auto auto;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    right: 30px;
                    top: 30px;
                    height: 24px;
                    width: 24px;
                    cursor: pointer;
                }

                &__close-cross-container:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }

            &__acknowledgement-container {
                border: 1px solid #c8d3de;
                border-radius: 6px;
                display: grid;
                grid-template-columns: 1fr 6fr;
                padding: 10px;
                margin-top: 25px;
                height: 80px;
                align-content: center;

                &__check {
                    margin: 0 auto auto;
                    border-radius: 4px;
                    height: 16px;
                    width: 16px;
                }

                &__text {
                    font-family: sans-serif;
                }
            }

            &__acknowledgement-buttons {
                display: flex;
                padding-top: 25px;
            }

            &__body-container {
                display: grid;
                grid-template-columns: 1fr 6fr;
                grid-template-rows: auto auto auto auto auto auto;
                width: 100%;
                padding-top: 10px;

                &__type-icon {
                    grid-column: 1;
                    grid-row: 1;
                }

                &__passphrase {
                    margin-top: 20px;
                    width: 100%;
                    background: #fff;
                    border: 1px solid #c8d3de;
                    box-sizing: border-box;
                    border-radius: 4px;
                    height: 40px;
                    font-size: 17px;
                    padding: 10px;
                }

                &__thumb-icon {
                    grid-column: 1;
                    grid-row: 2;
                }

                &__subtext-thumb {
                    grid-column: 2;
                    grid-row: 2;
                }

                &__type {
                    grid-column: 2;
                    grid-row: 1;
                    display: flex;
                    flex-direction: column;

                    &__type-container {
                        display: flex;
                        flex-direction: row;
                        align-items: center;
                    }
                }

                &__encrypt {
                    width: 100%;
                    text-align: left;
                    display: grid;
                    grid-template-columns: 1fr 6fr 1fr;
                    margin-top: 15px;
                    row-gap: 4ch;
                    grid-template-rows: 2fr 2fr;
                    padding-top: 10px;
                }

                &__name-icon {
                    grid-column: 1;
                    grid-row: 2;
                }

                &__name {
                    grid-column: 2;
                    grid-row: 2;
                    display: flex;
                    flex-direction: column;

                    &__input {
                        background: #fff;
                        border: 1px solid #c8d3de;
                        box-sizing: border-box;
                        border-radius: 4px;
                        height: 40px;
                        font-size: 17px;
                        padding: 10px;
                    }
                }

                &__permissions-icon {
                    grid-column: 1;
                    grid-row: 3;
                }

                &__permissions {
                    grid-column: 2;
                    grid-row: 3;
                    display: flex;
                    flex-direction: column;
                }

                &__buckets-icon {
                    grid-column: 1;
                    grid-row: 4;
                }

                &__buckets {
                    grid-column: 2;
                    grid-row: 4;
                    display: flex;
                    flex-direction: column;

                    &__bucket-bullets {
                        display: flex;
                        align-items: center;
                        max-width: 100%;
                        flex-wrap: wrap;

                        &__container {
                            display: flex;
                            margin-top: 5px;
                        }
                    }
                }

                &__date-icon {
                    grid-column: 1;
                    grid-row: 5;
                }

                &__duration {
                    grid-column: 2;
                    grid-row: 5;
                    display: flex;
                    flex-direction: column;

                    &__text {
                        color: #56606d;
                        text-decoration: underline;
                        font-family: sans-serif;
                        cursor: pointer;
                    }
                }

                &__notes-icon {
                    grid-column: 1;
                    grid-row: 6;
                }

                &__notes {
                    grid-column: 2;
                    grid-row: 6;
                    display: flex;
                    flex-direction: column;
                }

                & div {
                    padding-bottom: 10px;
                }
            }

            &__divider {
                height: 1px;
                background-color: #dadfe7;
                margin: 10px auto 0;
                width: 90%;
            }

            &__footer-container {
                display: flex;
                width: 100%;
                justify-content: space-evenly;
                padding-top: 25px;

                &__learn-more-button {
                    padding: 0 15px;
                }

                &__copy-button {
                    width: 49% !important;
                    margin-right: 10px;
                }

                &__download-button {
                    width: 49% !important;
                }

                &__encrypt-button {
                    padding: 0 15px;
                }
            }
        }
    }

    ::v-deep .buckets-selection {
        margin-left: 0;
        height: 30px;
        border: 1px solid #c8d3de;
    }

    ::v-deep .buckets-selection__toggle-container {
        padding: 10px 20px;
    }

    .permissions-chevron-up {
        @include chevron;

        transform: rotate(-90deg);
    }

    .permissions-chevron-down {
        @include chevron;
    }

    .tooltip-icon {
        width: 14px;
        height: 14px;
        cursor: pointer;
    }

    .tooltip-text {
        text-align: center;
        font-weight: 500;
    }

    a {
        color: #fff;
        text-decoration: underline !important;
        cursor: pointer;
    }

    .access-tooltip {
        top: 52px;
        left: 94px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: 100%;

            @include tooltip-arrow;
        }
    }

    .s3-tooltip {
        top: 158px;
        left: 103px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: -8%;
            transform: rotate(180deg);

            @include tooltip-arrow;
        }
    }

    .api-tooltip {
        top: 186px;
        left: 78px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: -11%;
            transform: rotate(180deg);

            @include tooltip-arrow;
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
