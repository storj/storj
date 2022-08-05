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
                <span class="tooltip-text">Keys to upload, delete, and view your project's data.  <a class="tooltip-link" href="https://docs.storj.io/dcs/concepts/access/access-grants" target="_blank" rel="noreferrer noopener">Learn More</a></span>
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
                                id="access-grant-check"
                                v-model="checkedType"
                                value="access"
                                type="radio"
                                name="type"
                                :checked="checkedType === 'access'"
                            >
                            <label for="access-grant-check">
                                Access Grant
                            </label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                alt="tooltip icon"
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
                                alt="tooltip icon"
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
                                alt="tooltip icon"
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
                                picker-style="__access-date-container"
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
                <div class="access-grant__modal-container__footer-container">
                    <a href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key/" target="_blank" rel="noopener noreferrer">
                        <v-button
                            label="Learn More"
                            width="150px"
                            height="50px"
                            is-transparent="true"
                            font-size="16px"
                            class="access-grant__modal-container__footer-container__learn-more-button"
                        />
                    </a>
                    <v-button
                        :label="checkedType === 'api' ? 'Create Keys  ⟶' : 'Encrypt My Access  ⟶'"
                        font-size="16px"
                        width="auto"
                        height="50px"
                        class="access-grant__modal-container__footer-container__encrypt-button"
                        :on-press="checkedType === 'api' ? createAccessGrant : encryptClickAction"
                        :is-disabled="selectedPermissions.length === 0 || accessName === ''"
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
                        <div
                            v-if="!(encryptSelect === 'create' && (isPassphraseDownloaded || isPassphraseCopied))"
                            class="access-grant__modal-container__body-container__encrypt__item"
                        >
                            <div class="access-grant__modal-container__body-container__encrypt__item__left-area">
                                <AccessKeyIcon
                                    class="access-grant__modal-container__body-container__encrypt__item__icon"
                                    :class="{ selected: encryptSelect === 'generate' }"
                                />
                                <div class="access-grant__modal-container__body-container__encrypt__item__text">
                                    <h3>Generate Passphrase</h3>
                                    <p>Automatically Generate Seed</p>
                                </div>
                            </div>
                            <div class="access-grant__modal-container__body-container__encrypt__item__radio">
                                <input
                                    id="generate-check"
                                    v-model="encryptSelect"
                                    value="generate"
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
                        <div
                            v-if="!(encryptSelect && (isPassphraseDownloaded || isPassphraseCopied))"
                            id="divider"
                            class="access-grant__modal-container__body-container__encrypt__divider"
                            :class="{ 'in-middle': encryptSelect === 'generate' }"
                        />
                        <div
                            v-if="!(encryptSelect === 'generate' && (isPassphraseDownloaded || isPassphraseCopied))"
                            id="own"
                            :class="{ 'in-middle': encryptSelect === 'generate' }"
                            class="access-grant__modal-container__body-container__encrypt__item"
                        >
                            <div class="access-grant__modal-container__body-container__encrypt__item__left-area">
                                <ThumbPrintIcon
                                    class="access-grant__modal-container__body-container__encrypt__item__icon"
                                    :class="{ selected: encryptSelect === 'create' }"
                                />
                                <div class="access-grant__modal-container__body-container__encrypt__item__text">
                                    <h3>Create My Own Passphrase</h3>
                                    <p>Make it Personalized</p>
                                </div>
                            </div>
                            <div class="access-grant__modal-container__body-container__encrypt__item__radio">
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
                        <input
                            v-if="encryptSelect === 'create'"
                            v-model="passphrase"
                            type="text"
                            placeholder="Input Your Passphrase"
                            class="access-grant__modal-container__body-container__passphrase" :disabled="encryptSelect === 'generate'"
                            @input="resetSavedStatus"
                        >
                        <div
                            class="access-grant__modal-container__footer-container"
                            :class="{ 'in-middle': encryptSelect === 'generate' }"
                        >
                            <v-button
                                :label="isPassphraseCopied ? 'Copied' : 'Copy to clipboard'"
                                width="auto"
                                height="50px"
                                :is-transparent="!isPassphraseCopied"
                                :is-white-green="isPassphraseCopied"
                                class="access-grant__modal-container__footer-container__copy-button"
                                font-size="16px"
                                :on-press="onCopyPassphraseClick"
                                :is-disabled="passphrase.length < 1"
                            >
                                <template v-if="!isPassphraseCopied" #icon>
                                    <copy-icon class="button-icon" :class="{ active: passphrase }" />
                                </template>
                            </v-button>
                            <v-button
                                label="Download .txt"
                                font-size="16px"
                                width="auto"
                                height="50px"
                                class="access-grant__modal-container__footer-container__download-button"
                                :is-green-white="isPassphraseDownloaded"
                                :on-press="downloadPassphrase"
                                :is-disabled="passphrase.length < 1"
                            >
                                <template v-if="!isPassphraseDownloaded" #icon>
                                    <download-icon class="button-icon" />
                                </template>
                            </v-button>
                        </div>
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
                            :on-press="createAccessGrant"
                        />
                    </div>
                </div>
            </form>
            <!-- *********   Grant Created Modal  ********* -->
            <form v-if="accessGrantStep === 'grantCreated'">
                <div class="access-grant__modal-container__header-container">
                    <AccessGrantsIcon v-if="checkedType === 'access'" />
                    <S3Icon v-if="checkedType === 's3'" />
                    <CLIIcon v-if="checkedType === 'api'" />
                    <div class="access-grant__modal-container__header-container__close-cross-container" @click="onCloseClick">
                        <CloseCrossIcon />
                    </div>
                    <h2 class="access-grant__modal-container__header-container__title-complete">{{ accessName }} <br> Created</h2>
                </div>
                <div class="access-grant__modal-container__body-container__created">
                    <p>Now copy and save the {{ checkedText[checkedType][0] }} will only appear once. Click on the {{ checkedText[checkedType][1] }}</p>
                </div>
                <div v-if="checkedType === 'access'">
                    <div class="access-grant__modal-container__generated-credentials__label first">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Access Grant
                        </span>
                        <a
                            href="https://docs.storj.io/dcs/concepts/access/access-grants/"
                            target="_blank"
                        >
                            <img
                                class="tooltip-icon"
                                alt="tooltip icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                            >
                        </a>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ access }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(access)"
                        >
                    </div>
                </div>
                <div v-if="checkedType === 's3'">
                    <div class="access-grant__modal-container__generated-credentials__label first">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Access Key
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.accessKeyId }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(gatewayCredentials.accessKeyId)"
                        >
                    </div>
                    <div class="access-grant__modal-container__generated-credentials__label">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Secret Key
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.secretKey }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(gatewayCredentials.secretKey)"
                        >
                    </div>
                    <div class="access-grant__modal-container__generated-credentials__label">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Endpoint
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.endpoint }}
                        </span>
                        <img
                            class="clickable-image"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            target="_blank"
                            href="https://docs.storj.io/dcs/concepts/satellite/"
                            @click="onCopyClick(gatewayCredentials.endpoint)"
                        >
                    </div>
                </div>
                <div v-if="checkedType === 'api'">
                    <div class="access-grant__modal-container__generated-credentials__label first">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Satellite Address
                        </span>
                        <a
                            href="https://docs.storj.io/dcs/concepts/satellite/"
                            target="_blank"
                        >
                            <img
                                class="tooltip-icon"
                                alt="tooltip icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                            >
                        </a>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ satelliteAddress }}
                        </span>
                        <img
                            class="clickable-image"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            alt="copy icon"
                            @click="onCopyClick(satelliteAddress)"
                        >
                    </div>
                    <div class="access-grant__modal-container__generated-credentials__label">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            API Key
                        </span>
                        <a
                            href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key/"
                            target="_blank"
                        >
                            <img
                                class="tooltip-icon"
                                alt="tooltip icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                            >
                        </a>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ restrictedKey }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="../../../static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(restrictedKey)"
                        >
                    </div>
                </div>
                <div v-if="checkedType === 's3'" class="access-grant__modal-container__credential-buttons__container-s3">
                    <a
                        v-if="checkedType === 's3'"
                        href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway/"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        <v-button
                            label="Learn More"
                            width="150px"
                            height="50px"
                            is-transparent="true"
                            font-size="16px"
                            class="access-grant__modal-container__footer-container__learn-more-button"
                        />
                    </a>
                    <v-button
                        label="Download .txt"
                        font-size="16px"
                        width="182px"
                        height="50px"
                        class="access-grant__modal-container__credential-buttons__download-button"
                        :is-green-white="areCredentialsDownloaded"
                        :on-press="downloadCredentials"
                    />
                </div>
                <div v-if="checkedType !== 's3'" class="access-grant__modal-container__credential-buttons__container">
                    <v-button
                        :label="isAccessGrantCopied ? 'Copied' : 'Copy to clipboard'"
                        width="auto"
                        height="50px"
                        :is-transparent="!isAccessGrantCopied"
                        :is-white-green="isAccessGrantCopied"
                        class="access-grant__modal-container__footer-container__copy-button"
                        font-size="16px"
                        :on-press="onCopyAccessGrantClick"
                        :is-disabled="restrictedKey.length < 1"
                    >
                        <template v-if="!isAccessGrantCopied" #icon>
                            <copy-icon class="button-icon" :class="{ active: restrictedKey }" />
                        </template>
                    </v-button>
                    <v-button
                        label="Download .txt"
                        font-size="16px"
                        width="182px"
                        height="50px"
                        class="access-grant__modal-container__credential-buttons__download-button"
                        :is-green-white="areCredentialsDownloaded"
                        :on-press="downloadCredentials"
                    />
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
import AccessGrantsIcon from '@/../static/images/accessGrants/accessGrantsIcon.svg';
import CopyIcon from '../../../static/images/common/copy.svg';
import DownloadIcon from '../../../static/images/common/download.svg';
import CLIIcon from '@/../static/images/accessGrants/cli.svg';
import S3Icon from '@/../static/images/accessGrants/s3.svg';

// for future use when notes is implemented
// import NotesIcon from '@/../static/images/accessGrants/create-access_notes.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';
import { Download } from "@/utils/download";
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { generateMnemonic } from "bip39";
import { AccessGrant } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from "@/store/modules/buckets";
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { EdgeCredentials } from '@/types/accessGrants';

// TODO: a lot of code can be refactored/reused/split into modules
// @vue/component
@Component({
    components: {
        VButton,
        AccessGrantsIcon,
        CLIIcon,
        S3Icon,
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
        CopyIcon,
        DownloadIcon,
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
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
    public areKeysVisible = false;
    private readonly FIRST_PAGE = 1;

    /**
     * Stores access type that is selected and text changes based on type.
     */
    private checkedType = '';
    private checkedText = {access: ['Access Grant as it','information icon to learn more.'], s3: ['S3 credentials as they','Learn More button to access the documentation.'],api: ['Satellite Address and API Key as they','information icons to learn more.']};
    private areCredentialsDownloaded = false;
    private isAccessGrantCopied = false;

    /**
     * Global isLoading Variable
     **/
    private isLoading = false;

    /**
     * Handles which tooltip is hovered over and set/clear timeout when leaving hover.
     */
    public tooltipHover = '';
    public tooltipVisibilityTimer;

    /**
     * Handles permission types, which have been selected, and determining if all have been selected.
     */
    private showAllPermissions = {show: false, position: "up"};
    private permissionsList = ["Read","Write","List","Delete"];
    private checkedPermissions = {Read: false, Write: false, List: false, Delete: false};
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

    private accessName = '';
    public areBucketNamesFetching = true;
    private addDateSelected = false;

    /**
     * Created Access Grant
     */
    private createdAccessGrant;
    private createdAccessGrantName = "";
    private createdAccessGrantSecret = "";
    private access = "";

    public currentDate = new Date().toISOString();
    private worker: Worker;
    private restrictedKey = '';
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');


    /**
     * Checks which type was selected and retrieves buckets on mount.
     */
    public async mounted(): Promise<void> {
        this.checkedType = this.defaultType;
        this.setWorker();
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }
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

    public resetSavedStatus(): void {
        this.isPassphraseDownloaded = false;
        this.isPassphraseCopied = false;
        this.acknowledgementCheck = false;
        this.encryptSelect = 'create';
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
        }

        this.accessGrantStep = 'grantCreated';
    }

    /**
     * Downloads passphrase to .txt file
     */
    public downloadPassphrase(): void {
        this.isPassphraseDownloaded = true;
        Download.file(this.passphrase, `passphrase-${this.currentDate}.txt`)
    }

    /**
     * Downloads credentials to .txt file
     */
    public downloadCredentials(): void {
        let credentialMap = {
            access: [`access grant: ${this.access}`],
            s3: [`access key: ${this.gatewayCredentials.accessKeyId}\nsecret key: ${this.gatewayCredentials.secretKey}\nendpoint: ${this.gatewayCredentials.endpoint}`],
            api: [`satellite address: ${this.satelliteAddress}\nrestricted key: ${this.restrictedKey}`]
        }
        this.areCredentialsDownloaded = true;
        Download.file(credentialMap[this.checkedType], `${this.checkedType}-credentials-${this.currentDate}.txt`)
    }

    public onRadioInput(): void {
        this.isPassphraseCopied = false;
        this.isPassphraseDownloaded = false;
        this.passphrase = '';

        if (this.encryptSelect === "generate") {
            this.passphrase = generateMnemonic();
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

    public onCopyClick(item): void {
        this.$copyText(item);
        this.$notify.success(`credential was copied successfully`);
    }

    public onCopyPassphraseClick(): void {
        this.$copyText(this.passphrase);
        this.isPassphraseCopied = true;
        this.$notify.success(`Passphrase was copied successfully`);
    }

    public onCopyAccessGrantClick(): void {
        this.$copyText(this.restrictedKey);
        this.isAccessGrantCopied = true;
        this.$notify.success(`Access Grant was copied successfully`);
    }

    public backAction(): void {
        this.accessGrantStep = 'create';
        this.passphrase = '';
        this.resetSavedStatus();
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
        this.showAllPermissions.show = !this.showAllPermissions.show;
        this.showAllPermissions.position = this.showAllPermissions.show ? 'up' : 'down';
    }

    /**
     * Handles permissions All.
     */
    public toggleAllPermission(type): void {
        if (type === 'all' && !this.allPermissionsClicked) {
            this.allPermissionsClicked = true;
            this.selectedPermissions = this.permissionsList;
            this.checkedPermissions = { Read: true, Write: true, List: true, Delete: true }
            return
        } else if(type === 'all' && this.allPermissionsClicked) {
            this.allPermissionsClicked = false;
            this.selectedPermissions = [];
            this.checkedPermissions = { Read: false, Write: false, List: false, Delete: false };
            return
        } else if(this.checkedPermissions[type]) {
            this.checkedPermissions[type] = false;
            this.allPermissionsClicked = false;
            return;
        } else {
            this.checkedPermissions[type] = true;
            if(this.checkedPermissions.Read && this.checkedPermissions.Write && this.checkedPermissions.List && this.checkedPermissions.Delete) {
                this.allPermissionsClicked = true;
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

    /**
     * Returns generated gateway credentials from store.
     */
    public get gatewayCredentials(): EdgeCredentials {
        return this.$store.state.accessGrantsModule.gatewayCredentials;
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

    @mixin generated-text {
        margin-top: 20px;
        align-items: center;
        padding: 10px 16px;
        background: #ebeef1;
        border: 1px solid #c8d3de;
        border-radius: 7px;
    }

    p {
        font-weight: bold;
        padding-bottom: 10px;
    }

    label {
        margin-left: 8px;
        padding-right: 10px;
    }

    h2 {
        font-weight: 800;
        font-size: 28px;
    }

    form {
        width: 100%;
    }

    #own.in-middle {
        order: 5;
    }

    .blue-background {
        background: #d7e8ff;
    }

    .clickable-image {
        cursor: pointer;
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

            &__generated-passphrase {
                @include generated-text;
            }

            &__generated-credentials {
                @include generated-text;

                margin: 0 0 4px;
                display: flex;
                justify-content: space-between;

                &__text {
                    width: 90%;
                    text-overflow: ellipsis;
                    overflow-x: hidden;
                    white-space: nowrap;
                }

                &__label {
                    display: flex;
                    margin: 24px 0 8px;
                    align-items: center;

                    &.first {
                        margin-top: 8px;
                    }

                    &__text {
                        font-family: sans-serif;
                        font-size: 14px;
                        font-weight: 700;
                        line-height: 20px;
                        letter-spacing: 0;
                        text-align: left;
                        padding: 0 6px 0 0;
                    }
                }
            }

            &__credential-buttons {

                &__container-s3 {
                    display: flex;
                    justify-content: space-between;
                    margin: 15px 0;
                }

                &__container {
                    display: flex;
                    justify-content: space-between;
                    margin: 15px 0;
                }
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

                &__title-complete {
                    grid-column: 1;
                    margin-top: 10px;
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
                grid-row-gap: 24px;
                width: 100%;
                padding-top: 10px;
                margin-top: 24px;

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

                &__type {
                    grid-column: 2;
                    grid-row: 1;
                    display: flex;
                    flex-direction: column;

                    &__type-container {
                        display: flex;
                        flex-direction: row;
                        align-items: center;
                        margin-bottom: 10px;
                    }
                }

                &__encrypt {
                    width: 100%;
                    display: flex;
                    flex-flow: column;
                    align-items: center;
                    justify-content: center;
                    margin: 15px 0;

                    &__item {
                        display: flex;
                        align-items: center;
                        justify-content: space-between;
                        width: 100%;
                        height: 40px;
                        box-sizing: border-box;

                        &__left-area {
                            display: flex;
                            align-items: center;
                            justify-content: flex-start;
                        }

                        &__icon {
                            margin-right: 8px;

                            &.selected {

                                ::v-deep circle {
                                    fill: #e6edf7 !important;
                                }

                                ::v-deep path {
                                    fill: #003dc1 !important;
                                }
                            }
                        }

                        &__text {
                            display: flex;
                            flex-direction: column;
                            justify-content: space-between;
                            align-items: flex-start;
                            font-family: 'font_regular', sans-serif;
                            font-size: 12px;

                            h3 {
                                margin: 0 0 8px;
                                font-family: 'font_bold', sans-serif;
                                font-size: 14px;
                            }

                            p {
                                padding: 0;
                            }
                        }

                        &__radio {
                            display: flex;
                            align-items: center;
                            justify-content: center;
                            width: 10px;
                            height: 10px;
                        }
                    }

                    &__divider {
                        width: 100%;
                        height: 1px;
                        background: #ebeef1;
                        margin: 16px 0;

                        &.in-middle {
                            order: 4;
                        }
                    }
                }

                &__created {
                    width: 100%;
                    text-align: left;
                    display: grid;
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    margin-top: 15px;
                    row-gap: 4ch;
                    padding-top: 10px;

                    p {
                        font-style: normal;
                        font-weight: 400;
                        font-size: 14px;
                        line-height: 20px;
                        overflow-wrap: break-word;
                        text-align: left;
                    }
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
                    max-width: 238px;

                    &__input {
                        background: #fff;
                        border: 1px solid #c8d3de;
                        box-sizing: border-box;
                        border-radius: 6px;
                        height: 40px;
                        font-size: 17px;
                        padding: 10px;
                    }

                    &__input:focus {
                        border-color: #2683ff;
                    }
                }

                &__input:focus {
                    border-color: #2683ff;
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
                        color: #929fb1;
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
            }

            &__footer-container {
                display: flex;
                width: 100%;
                justify-content: flex-start;
                margin-top: 16px;

                & ::v-deep .container:first-of-type {
                    margin-right: 8px;
                }

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

                .in-middle {
                    order: 3;
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
        display: flex;
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

    .button-icon {
        margin-right: 5px;

        ::v-deep path,
        ::v-deep rect {
            stroke: white;
        }

        &.active {

            ::v-deep path,
            ::v-deep rect {
                stroke: #56606d;
            }
        }
    }

    .access-tooltip {
        top: 52px;
        left: 109px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: 100%;

            @include tooltip-arrow;
        }
    }

    .s3-tooltip {
        top: 158px;
        left: 118px;

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
        left: 94px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: -11%;
            transform: rotate(180deg);

            @include tooltip-arrow;
        }
    }

    .access-bucket-container {
        padding-bottom: 10px;
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
