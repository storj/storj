// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-access">
        <h2 class="create-access__title">Create Access</h2>
        <div class="create-access__fragment">
            <TypesIcon />
            <div class="create-access__fragment__wrap">
                <p class="create-access__fragment__wrap__label">Type</p>
                <div class="create-access__fragment__wrap__type-container">
                    <label class="checkmark-container">
                        <input
                            id="access-grant-check"
                            type="checkbox"
                            :checked="getIsChecked('access')"
                            @change="event => checkChanged(event, 'access')"
                        >
                        <span class="checkmark" />
                    </label>
                    <label for="access-grant-check">
                        Access Grant
                    </label>
                    <img
                        class="tooltip-icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('access','over')"
                        @mouseleave="toggleTooltipHover('access','leave')"
                    >
                    <div
                        v-if="tooltipHover === 'access'"
                        class="access-tooltip"
                        @mouseover="toggleTooltipHover('access','over')"
                        @mouseleave="toggleTooltipHover('access','leave')"
                    >
                        <span class="tooltip-text">Keys to upload, delete, and view your project's data.  <a class="tooltip-link" href="https://docs.storj.io/dcs/concepts/access/access-grants" target="_blank" rel="noreferrer noopener" @click="trackPageVisit('https://docs.storj.io/dcs/concepts/access/access-grants')">Learn More</a></span>
                    </div>
                </div>
                <div class="create-access__fragment__wrap__type-container">
                    <label class="checkmark-container">
                        <input
                            id="s3-check"
                            type="checkbox"
                            :checked="getIsChecked('s3')"
                            @change="event => checkChanged(event, 's3')"
                        >
                        <span class="checkmark" />
                    </label>
                    <label for="s3-check">
                        S3 Credentials
                    </label>
                    <img
                        class="tooltip-icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('s3','over')"
                        @mouseleave="toggleTooltipHover('s3','leave')"
                    >
                    <div
                        v-if="tooltipHover === 's3'"
                        class="s3-tooltip"
                        @mouseover="toggleTooltipHover('s3','over')"
                        @mouseleave="toggleTooltipHover('s3','leave')"
                    >
                        <span class="tooltip-text">Generates access key, secret key, and endpoint to use in your S3-supporting application. <a class="tooltip-link" href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway" target="_blank" rel="noreferrer noopener" @click="trackPageVisit('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway')">Learn More</a></span>
                    </div>
                </div>
                <div class="create-access__fragment__wrap__type-container">
                    <label class="checkmark-container">
                        <input
                            id="api-check"
                            type="checkbox"
                            :checked="getIsChecked('api')"
                            @change="event => checkChanged(event, 'api')"
                        >
                        <span class="checkmark" />
                    </label>
                    <label for="api-check">
                        API Access
                    </label>
                    <img
                        class="tooltip-icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('api','over')"
                        @mouseleave="toggleTooltipHover('api','leave')"
                    >
                    <div
                        v-if="tooltipHover === 'api'"
                        class="api-tooltip"
                        @mouseover="toggleTooltipHover('api','over')"
                        @mouseleave="toggleTooltipHover('api','leave')"
                    >
                        <span class="tooltip-text">Creates access grant to run in the command line.  <a class="tooltip-link" href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token" target="_blank" rel="noreferrer noopener" @click="trackPageVisit('https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token')">Learn More</a></span>
                    </div>
                </div>
            </div>
        </div>
        <div class="create-access__fragment">
            <NameIcon />
            <div class="create-access__fragment__wrap">
                <p class="create-access__fragment__wrap__label">Name</p>
                <input
                    v-model="accessName"
                    type="text"
                    placeholder="Input Access Name" class="create-access__fragment__wrap__input"
                >
            </div>
        </div>
        <div class="create-access__fragment">
            <PermissionsIcon />
            <div class="create-access__fragment__wrap">
                <p class="create-access__fragment__wrap__label">Permissions</p>
                <div class="create-access__fragment__wrap__permission">
                    <label class="checkmark-container">
                        <input
                            id="permissions__all-check"
                            type="checkbox"
                            :checked="allPermissionsClicked"
                            @click="toggleAllPermission('all')"
                        >
                        <span class="checkmark" />
                    </label>
                    <label for="permissions__all-check">
                        All
                    </label>
                    <Chevron :class="`permissions-chevron-${showAllPermissions.position}`" @click="togglePermissions" />
                </div>
                <div v-if="showAllPermissions.show">
                    <div v-for="item in permissionsList" :key="item" class="create-access__fragment__wrap__permission">
                        <label class="checkmark-container">
                            <input
                                :id="`permissions__${item}-check`"
                                type="checkbox"
                                :value="item"
                                :checked="checkedPermissions[item]"
                                @click="toggleAllPermission(item)"
                            >
                            <span class="checkmark" />
                        </label>
                        <label :for="`permissions__${item}-check`">{{ item }}</label>
                    </div>
                </div>
            </div>
        </div>
        <div class="create-access__fragment">
            <BucketsIcon />
            <div class="create-access__fragment__wrap">
                <p class="create-access__fragment__wrap__label">Buckets</p>
                <div>
                    <BucketsSelection
                        class="access-bucket-container"
                        :show-scrollbar="true"
                    />
                </div>
                <div class="create-access__fragment__wrap__bucket-bullets">
                    <div
                        v-for="(name, index) in selectedBucketNames"
                        :key="index"
                        class="create-access__fragment__wrap__bucket-bullets__container"
                    >
                        <BucketNameBullet :name="name" />
                    </div>
                </div>
            </div>
        </div>
        <div class="create-access__fragment">
            <DateIcon />
            <div class="create-access__fragment__wrap">
                <p class="create-access__fragment__wrap__label">Duration</p>
                <div v-if="addDateSelected">
                    <DurationSelection
                        container-style="access-date-container"
                        text-style="access-date-text"
                        picker-style="__access-date-container"
                    />
                </div>
                <div
                    v-else
                    class="create-access__fragment__wrap__text"
                    @click="addDateSelected = true"
                >
                    Add Date (optional)
                </div>
            </div>
        </div>
        <div class="create-access__buttons">
            <a href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key" target="_blank" rel="noopener noreferrer" @click="trackPageVisit('https://docs.storj.io/dcs/concepts/access/access-grants/api-key')">
                <v-button
                    label="Learn More"
                    height="48px"
                    :is-transparent="true"
                    font-size="14px"
                    class="create-access__buttons__button"
                />
            </a>
            <v-button
                :label="checkedTypes.includes('api') ? 'Create Keys  ⟶' : 'Encrypt My Access  ⟶'"
                font-size="14px"
                height="48px"
                :on-press="checkedTypes.includes('api') ? propagateInfo : encryptClickAction"
                :is-disabled="!selectedPermissions.length || !accessName || !checkedTypes.length"
                class="create-access__buttons__button"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onMounted, reactive, ref } from 'vue';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify, useStore } from '@/utils/hooks';

import VButton from '@/components/common/VButton.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import BucketNameBullet from '@/components/accessGrants/permissions/BucketNameBullet.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';

import DateIcon from '@/../static/images/accessGrants/create-access_date.svg';
import TypesIcon from '@/../static/images/accessGrants/create-access_type.svg';
import NameIcon from '@/../static/images/accessGrants/create-access_name.svg';
import PermissionsIcon from '@/../static/images/accessGrants/create-access_permissions.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';
import BucketsIcon from '@/../static/images/accessGrants/create-access_buckets.svg';

type ShowPermissions = {
    show: boolean,
    position: string
}

type Permissions = {
    Read: boolean,
    Write: boolean,
    List: boolean,
    Delete: boolean
}

const props = withDefaults(defineProps<{ checkedType?: string; }>(), { checkedType: '' });

const emit = defineEmits(['close-modal', 'propagateInfo', 'encrypt']);

const store = useStore();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const checkedTypes = ref<string[]>([]);
const accessName = ref<string>('');
const selectedPermissions = ref<string[]>([]);
const allPermissionsClicked = ref<boolean>(false);
const permissionsList = ref<string[]>(['Read','Write','List','Delete']);
const addDateSelected = ref<boolean>(false);
const tooltipHover = ref<string>('');
const tooltipVisibilityTimer = ref<ReturnType<typeof setTimeout> | null>();

let checkedPermissions = reactive<Permissions>({ Read: false, Write: false, List: false, Delete: false });
let showAllPermissions = reactive<ShowPermissions>({ show: false, position: 'up' });

const accessGrantsList = computed((): AccessGrant[] => {
    return store.state.accessGrantsModule.page.accessGrants;
});

/**
 * Retrieves selected buckets for bucket bullets.
 */
const selectedBucketNames = computed((): string[] => {
    return store.state.accessGrantsModule.selectedBucketNames;
});

function onCloseClick(): void {
    store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
    emit('close-modal');
}

/**
 * Whether some type of access is selected
 * @param type
 */
function getIsChecked(type: string): boolean {
    return checkedTypes.value.includes(type);
}

function checkChanged(event: { target: { checked: boolean } }, type: string): void {
    const isSelected = event.target.checked;
    if (type === 'api') {
        if (isSelected) {
            checkedTypes.value = ['api'];
        } else {
            checkedTypes.value = checkedTypes.value.filter(t => t !== 'api');
        }
    } else {
        if (isSelected) {
            checkedTypes.value = checkedTypes.value.filter(t => t !== 'api');
            checkedTypes.value.push(type);
        } else {
            checkedTypes.value = checkedTypes.value.filter(t => t !== type);
        }
    }
}

/**
 * propagates selected info to parent on flow progression.
 */
function propagateInfo(): void {
    if (!checkedTypes.value.length) return;

    const payloadObject  = {
        'checkedType': checkedTypes.value.join(','),
        'accessName': accessName.value,
        'selectedPermissions': selectedPermissions.value,
    };

    emit('propagateInfo', payloadObject, checkedTypes.value.join(','));
}

/**
 * Toggles permissions list visibility.
 */
function togglePermissions(): void {
    showAllPermissions.show = !showAllPermissions.show;
    showAllPermissions.position = showAllPermissions.show ? 'up' : 'down';
}

function encryptClickAction(): void {
    let mappedList = accessGrantsList.value.map((key) => (key.name));
    if (mappedList.includes(accessName.value)) {
        notify.error(`validation: An API Key with this name already exists in this project, please use a different name`, AnalyticsErrorEventSource.CREATE_AG_FORM);
        return;
    } else if (!checkedTypes.value.includes('api')) {
        // emit event here
        propagateInfo();
        emit('encrypt');
    }
    analytics.eventTriggered(AnalyticsEvent.ENCRYPT_MY_ACCESS_CLICKED);
}

function toggleAllPermission(type): void {
    if (type === 'all') {
        allPermissionsClicked.value = !allPermissionsClicked.value;
        selectedPermissions.value = allPermissionsClicked.value ? permissionsList.value : [];

        for (const permission in checkedPermissions) {
            checkedPermissions[permission] = allPermissionsClicked.value;
        }

        return;
    }

    if (checkedPermissions[type]) {
        checkedPermissions[type] = false;
        allPermissionsClicked.value = false;
    } else {
        checkedPermissions[type] = true;
        if (checkedPermissions.Read && checkedPermissions.Write && checkedPermissions.List && checkedPermissions.Delete) {
            allPermissionsClicked.value = true;
        }
    }
}

/**
 * Toggles tooltip visibility.
 */
function toggleTooltipHover(type, action): void {
    if (tooltipHover.value === '' && action === 'over') {
        tooltipHover.value = type;
        return;
    } else if (tooltipHover === type && action === 'leave') {
        tooltipVisibilityTimer.value = setTimeout(() => {
            tooltipHover.value = '';
        },750);
        return;
    } else if (tooltipHover.value === type && action === 'over') {
        tooltipVisibilityTimer.value && clearTimeout(tooltipVisibilityTimer.value);
        return;
    } else if (tooltipHover !== type) {
        tooltipVisibilityTimer.value && clearTimeout(tooltipVisibilityTimer.value);
        tooltipHover.value = type;
    }
}

/**
 * Sends "trackPageVisit" event to segment and opens link.
 */
function trackPageVisit(link: string): void {
    analytics.pageVisit(link);
}

onMounted(() => {
    showAllPermissions.show = false;
    showAllPermissions.position = 'down';
});

onBeforeMount(() => {
    if (props.checkedType) checkedTypes.value = [props.checkedType];
});
</script>

<style scoped lang="scss">
    @mixin tooltip-container {
        position: absolute;
        background: var(--c-grey-6);
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
        border-top-color: var(--c-grey-6);
        border-bottom: 0;
        margin-left: -20px;
        margin-bottom: -20px;
    }

    p {
        font-weight: bold;
        padding-bottom: 10px;
    }

    svg {
        min-width: 40px;
    }

    label {
        padding-right: 10px;
    }

    @mixin chevron {
        padding-left: 4px;
        transition: transform 0.3s;
        min-width: unset;
    }

    .permissions-chevron-up {
        @include chevron;

        transform: rotate(-180deg);
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

    .create-access {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        padding: 32px;
        font-family: 'font_regular', sans-serif;
        max-width: 346px;

        @media screen and (max-width: 390px) {
            padding: 32px 12px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            margin-bottom: 24px;
        }

        &__fragment {
            display: flex;
            align-items: flex-start;
            margin-bottom: 16px;
            width: 100%;

            &__wrap {
                display: flex;
                flex-direction: column;
                margin-left: 16px;
                width: 100%;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: #000;
                    text-align: left;
                }

                &__type-container {
                    display: flex;
                    align-items: center;
                    margin-bottom: 10px;
                }

                &__input {
                    background: #fff;
                    border: 1px solid var(--c-grey-4);
                    box-sizing: border-box;
                    border-radius: 6px;
                    font-size: 14px;
                    padding: 10px;
                    width: 100%;
                }

                &__input:focus {
                    border-color: #2683ff;
                }

                &__permission {
                    display: flex;
                    align-items: center;
                    margin-bottom: 10px;
                }

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

                &__text {
                    color: var(--c-grey-5);
                    text-decoration: underline;
                    cursor: pointer;
                    text-align: left;
                }
            }
        }

        &__buttons {
            display: flex;
            width: 100%;
            justify-content: flex-start;
            margin-top: 16px;
            column-gap: 8px;

            @media screen and (max-width: 390px) {
                flex-direction: column;
                column-gap: unset;
                row-gap: 8px;
            }

            &__button {
                padding: 0 15px;

                @media screen and (max-width: 390px) {
                    width: unset !important;
                }
            }
        }
    }

    :deep(.buckets-selection) {
        margin-left: 0;
        height: 40px;
        border: 1px solid var(--c-grey-4);
    }

    :deep(.buckets-selection__toggle-container) {
        padding: 10px 20px;
    }

    :deep(.buckets-dropdown__container__all) {
        text-align: left;
    }

    .access-tooltip {
        top: 66px;
        left: 104px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: 100%;

            @include tooltip-arrow;
        }
    }

    .s3-tooltip {
        top: 182px;
        left: 113px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: -8%;
            transform: rotate(180deg);

            @include tooltip-arrow;
        }
    }

    .api-tooltip {
        top: 215px;
        left: 90px;

        @include tooltip-container;

        &:after {
            left: 50%;
            top: -11%;
            transform: rotate(180deg);

            @include tooltip-arrow;
        }
    }

    .checkmark-container {
        position: relative;
        height: 21px;
        width: 21px;
        cursor: pointer;
        font-size: 22px;
        user-select: none;
        outline: none;
    }

    .checkmark-container input {
        position: absolute;
        opacity: 0;
        cursor: pointer;
        height: 0;
        width: 0;
    }

    .checkmark {
        position: absolute;
        top: 0;
        left: 0;
        height: 21px;
        width: 21px;
        border: 2px solid #afb7c1;
        border-radius: 4px;
    }

    .checkmark-container:hover input ~ .checkmark {
        background-color: white;
    }

    .checkmark-container input:checked ~ .checkmark {
        border: 2px solid #376fff;
        background-color: var(--c-blue-3);
    }

    .checkmark:after {
        content: '';
        position: absolute;
        display: none;
    }

    .checkmark-container .checkmark:after {
        left: 7px;
        top: 3px;
        width: 5px;
        height: 10px;
        border: solid white;
        border-width: 0 3px 3px 0;
        transform: rotate(45deg);
    }

    .checkmark-container input:checked ~ .checkmark:after {
        display: block;
    }
</style>
