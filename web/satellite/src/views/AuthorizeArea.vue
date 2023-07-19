// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="authorize-area">
        <div class="authorize-area__logo-wrapper">
            <LogoIcon class="logo" @click="location.reload()" />
        </div>

        <div class="authorize-area__content-area">
            <div v-if="requestErr" class="authorize-area__content-area__container">
                <p>{{ requestErr }}</p>
            </div>
            <div v-else class="authorize-area__content-area__container">
                <p v-if="client.appLogoURL" class="authorize-area__content-area__client-app-logo">
                    <img :alt="client.appName" :src="client.appLogoURL">
                </p>

                <p class="authorize-area__content-area__client-app">
                    {{ client.appName }} would like permission to:
                </p>

                <div class="authorize-area__permissions-area">
                    <div class="authorize-area__permissions-area__container">
                        <p class="authorize-area__permissions-area__header">Verify your Storj Identity</p>
                        <p>Access and view your account info.</p>
                    </div>
                    <div class="authorize-area__permissions-area__container">
                        <p class="authorize-area__permissions-area__header">Sync data to Storj DCS</p>
                        <p>Automatically send updates to:</p>

                        <div class="authorize-area__input-wrapper">
                            <VInput
                                label="Project"
                                role-description="project"
                                :error="projectErr"
                                :options-list="Object.keys(projects)"
                                @setData="setProject"
                            />
                        </div>

                        <div class="authorize-area__input-wrapper">
                            <VInput
                                label="Bucket"
                                role-description="bucket"
                                :error="bucketErr"
                                :value="selectedBucketName"
                                :options-list="buckets"
                                @setData="setBucket"
                            />
                            <div v-if="!bucketExists" class="info-box">
                                <p class="info-box__message">
                                    This bucket will be created.
                                </p>
                            </div>
                        </div>

                        <div class="authorize-area__input-wrapper">
                            <VInput
                                label="Passphrase"
                                role-description="passphrase"
                                placeholder="Passphrase"
                                :error="passphraseErr"
                                is-password
                                @setData="setPassphrase"
                            />
                        </div>
                    </div>
                    <div class="authorize-area__permissions-area__container">
                        <p class="authorize-area__permissions-area__container__header">Perform the following actions</p>
                        <p>{{ actions }} objects.</p>
                    </div>
                </div>

                <form method="post">
                    <input v-model="oauthData.client_id" type="hidden" name="client_id">
                    <input v-model="oauthData.redirect_uri" type="hidden" name="redirect_uri">
                    <input v-model="oauthData.response_type" type="hidden" name="response_type">
                    <input v-model="oauthData.state" type="hidden" name="state">
                    <input v-model="scope" type="hidden" name="scope">

                    <input class="authorize-area__content-area__container__button" :class="{ 'disabled-button': !valid }" type="submit" :disabled="!valid" value="Authorize">
                    <p class="authorize-area__content-area__container__cancel" @click.prevent="onDeny">Cancel</p>
                </form>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Validator } from '@/utils/validation';
import { RouteConfig } from '@/types/router';
import { Project } from '@/types/projects';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { OAuthClient, OAuthClientsAPI } from '@/api/oauthClients';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VInput from '@/components/common/VInput.vue';

import LogoIcon from '@/../static/images/logo.svg';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
const oauthClientsAPI = new OAuthClientsAPI();
const validPerms = {
    'list': true,
    'read': true,
    'write': true,
    'delete': true,
};

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const worker = ref<Worker | null>(null);
const valid = ref<boolean>(false);
const bucketExists = ref<boolean>(false);
const actions = ref<string>('');
const clientKey = ref<string>('');
const selectedProjectID = ref<string>('');
const selectedBucketName = ref<string>('');
const providedPassphrase = ref<string>('');
const scope = ref<string>('');
const buckets = ref<string[]>([]);
const requestErr = ref<string>('');
const projectErr = ref<string>('');
const bucketErr = ref<string>('');
const passphraseErr = ref<string>('');
const projects = ref<Record<string, Project>>({});
const client = ref<OAuthClient>({
    id: '',
    redirectURL: '',
    appName: '',
    appLogoURL: '',
});
const oauthData = ref<{
    client_id?: string;
    redirect_uri?: string;
    state?: string;
    response_type?: string;
    scope?: string;
}>({});

function slugify(name: string): string {
    name = name.toLowerCase();
    name = name.replace(/\s+/g, '-');
    return name;
}

function formatObjectPermissions(scope: string): string {
    const scopes = scope.split(' ');
    const perms: string[] = [];

    for (const scope of scopes) {
        if (scope.startsWith('object:')) {
            const perm = scope.substring('object:'.length);
            if (validPerms[perm]) {
                perms.push(perm);
            }
        }
    }

    perms.sort();

    if (perms.length === 0) {
        return '';
    } else if (perms.length === 1) {
        return perms[0];
    } else if (perms.length === 2) {
        return `${perms[0]} and ${perms[1]}`;
    }

    return `${perms.slice(0, perms.length - 1).join(', ')}, and ${perms[perms.length - 1]}`;
}

async function ensureLogin(): Promise<void> {
    try {
        await usersStore.getUser();
    } catch (error) {
        if (!(error instanceof ErrorUnauthorized)) {
            appStore.changeState(FetchState.ERROR);
            notify.notifyError(error, null);
        }

        const query = new URLSearchParams(oauthData.value).toString();
        const path = `${RouteConfig.Authorize.path}?${query}#${clientKey.value}`;

        analytics.pageVisit(`${RouteConfig.Login.path}?return_url=${encodeURIComponent(path)}`);
        await router.push(`${RouteConfig.Login.path}?return_url=${encodeURIComponent(path)}`);
    }
}

async function ensureWorker(): Promise<void> {
    try {
        agStore.stopWorker();
        await agStore.startWorker();
    } catch (error) {
        await notify.error(`Unable to set access grants wizard. ${error.message}`, null);
        return;
    }

    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => notify.error(error.message, null);
    }
}

async function verifyClientConfiguration(): Promise<void> {
    const clientID: string = oauthData.value.client_id ?? '';
    const redirectURL: string = oauthData.value.redirect_uri ?? '';
    const state: string = oauthData.value.state ?? '';
    const responseType: string = oauthData.value.response_type ?? '';
    const scope: string = oauthData.value.scope ?? '';

    if (!clientID || !redirectURL) {
        requestErr.value = 'Both client_id and redirect_uri must be provided.';
        return;
    }

    let oAuthClient: OAuthClient;
    try {
        oAuthClient = await oauthClientsAPI.get(clientID);
    } catch (error) {
        requestErr.value = error.message;
        return;
    }

    let err: { [key: string]: string } | null = null;

    if (!state || !responseType || !scope) {
        err = {
            error_description: 'The request is missing a required parameter (state, response_type, or scope).',
        };
    } else if (!clientKey.value) {
        err = {
            error_description: 'An encryption key must be provided in the fragment of the request.',
        };
    } else if (!redirectURL.startsWith(oAuthClient.redirectURL)) {
        err = {
            error_description: 'The provided redirect url does not match the one in our system.',
        };
    }

    if (err) {
        location.href = `${redirectURL}?${(new URLSearchParams(err)).toString()}`;
        return;
    }

    client.value = { ...oAuthClient };

    // initialize the form
    setBucket(slugify(client.value.appName));
    actions.value = formatObjectPermissions(scope);
}

async function loadProjects(): Promise<void> {
    await projectsStore.getProjects();

    const newProjects: Record<string, Project> = {};
    for (const project of projectsStore.projects) {
        newProjects[project.name] = project;
    }

    projects.value = { ...newProjects };
}

async function setProject(value: string): Promise<void> {
    if (!projects.value[value]) {
        projectErr.value = 'project does not exist';
        return;
    }

    const projectID = projectsStore.state.selectedProject.id;

    projectsStore.selectProject(projects.value[value].id);
    await bucketsStore.getAllBucketsNames(projectID);

    selectedProjectID.value = projectID;
    buckets.value = bucketsStore.state.allBucketNames.sort();

    setBucket(selectedBucketName.value);
}

function setBucket(value: string): void {
    selectedBucketName.value = value;
    bucketExists.value = selectedProjectID.value.length === 0 || value.length === 0 || buckets.value.includes(value);

    setScope();
}

function setPassphrase(value: string): void {
    providedPassphrase.value = value;

    setScope();
}

async function setScope(): Promise<void> {
    if (!validate() || !worker.value) {
        return;
    }

    worker.value.postMessage({
        'type': 'DeriveAndEncryptRootKey',
        'passphrase': providedPassphrase.value,
        'projectID': selectedProjectID.value,
        'aesKey': clientKey.value,
    });

    const event: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });

    if (event.data.error) {
        await notify.error(event.data.error, null);
        return;
    }

    const oAuthScope = oauthData.value.scope,
        project = selectedProjectID.value,
        bucket = selectedBucketName.value,
        cubbyhole = event.data.value;

    scope.value = `${oAuthScope} project:${project} bucket:${bucket} cubbyhole:${cubbyhole}`;
}

async function onDeny(): Promise<void> {
    location.href = `${oauthData.value.redirect_uri}?${new URLSearchParams({
        error_description: 'The resource owner or authorization server denied the request',
    }).toString()}`;
}

function validate(): boolean {
    projectErr.value = '';
    bucketErr.value = '';
    passphraseErr.value = '';

    if (selectedProjectID.value === '') {
        projectErr.value = 'Missing project.';
    }

    if (!Validator.bucketName(selectedBucketName.value)) {
        bucketErr.value = 'Name must contain only lowercase latin characters, numbers, a hyphen or a period';
    }

    if (providedPassphrase.value === '') {
        passphraseErr.value = 'A passphrase must be provided.';
    }

    valid.value = projectErr.value === '' &&
        bucketErr.value === '' &&
        passphraseErr.value === '';

    return valid.value;
}

/**
 * Lifecycle hook after initial render.
 * Makes activated banner visible on successful account activation.
 */
onMounted(async () => {
    oauthData.value = { ...route.query };
    clientKey.value = route.hash ? route.hash.substring(1) : '';

    await ensureLogin();
    await ensureWorker();

    await verifyClientConfiguration();
    if (requestErr.value) {
        return;
    }

    await loadProjects();
});
</script>

<style scoped lang="scss">
    .authorize-area {
        display: flex;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        position: fixed;
        inset: 0;
        min-height: 100%;
        overflow-y: scroll;

        .info-box {
            background-color: #e9f3ff;
            border-radius: 6px;
            padding: 20px;
            margin-top: 25px;
            width: 100%;
            box-sizing: border-box;

            &.error {
                background-color: #fff9f7;
                border: 1px solid #f84b00;
            }

            &__header {
                display: flex;
                align-items: center;

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    color: #1b2533;
                    margin-left: 15px;
                }
            }

            &__message {
                font-size: 16px;
                color: #1b2533;
            }
        }

        &__permissions-area {
            background-color: var(--c-grey-1);
            border: 1px solid var(--c-grey-3);
            padding: 10px 9px 10px 24px;
            border-radius: 8px;

            &__header {
                font-size: 18px;
                margin-bottom: 4px;
            }

            &__container {
                padding: 24px 0;
                border-bottom: 1px solid var(--c-grey-3);

                p {
                    line-height: 24px;
                    vertical-align: middle;
                }

                &:last-of-type {
                    border-bottom: none;
                }
            }
        }

        &__logo-wrapper {
            text-align: center;
            margin: 70px 0;
        }

        &__divider {
            margin: 0 20px;
            height: 22px;
            width: 2px;
            background-color: #acbace;
        }

        &__input-wrapper {
            margin-top: 20px;
            padding-right: 24px;
            width: 100%;
            box-sizing: border-box;
        }

        &__expand {
            display: flex;
            align-items: center;
            cursor: pointer;
            position: relative;

            &__value {
                font-size: 16px;
                line-height: 21px;
                color: #acbace;
                margin-right: 10px;
                font-family: 'font_regular', sans-serif;
                font-weight: 700;
            }

            &__dropdown {
                position: absolute;
                top: 35px;
                left: 0;
                background-color: #fff;
                z-index: 1000;
                border: 1px solid #c5cbdb;
                box-shadow: 0 8px 34px rgb(161 173 185 / 41%);
                border-radius: 6px;
                min-width: 250px;

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;
                    padding: 12px 25px;
                    font-size: 14px;
                    line-height: 20px;
                    color: #7e8b9c;
                    cursor: pointer;
                    text-decoration: none;

                    &__name {
                        font-family: 'font_bold', sans-serif;
                        margin-left: 15px;
                        font-size: 14px;
                        line-height: 20px;
                        color: #7e8b9c;
                    }

                    &:hover {
                        background-color: #f2f2f6;
                    }
                }
            }
        }

        &__content-area {
            background-color: #f5f6fa;
            padding: 0 20px;
            margin-bottom: 50px;
            display: flex;
            flex-direction: column;
            align-items: center;
            border-radius: 20px;
            box-sizing: border-box;

            &__activation-banner {
                padding: 20px;
                background-color: rgb(39 174 96 / 10%);
                border: 1px solid #27ae60;
                color: #27ae60;
                border-radius: 6px;
                width: 570px;
                margin-bottom: 30px;

                &__message {
                    font-size: 16px;
                    line-height: 21px;
                    margin: 0;
                }
            }

            &__client-app-logo {
                text-align: center;
                margin-bottom: 60px;
            }

            &__client-app {
                text-align: center;
                font-size: 22px;
                font-weight: bold;
                margin-bottom: 16px;
            }

            &__container {
                display: flex;
                flex-direction: column;
                padding: 60px 80px;
                background-color: #fff;
                width: 610px;
                border-radius: 20px;
                box-sizing: border-box;
                margin-bottom: 20px;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        line-height: 49px;
                        letter-spacing: -0.1007px;
                        color: #252525;
                        font-family: 'font_bold', sans-serif;
                        font-weight: 800;
                    }

                    &__satellite {
                        font-size: 16px;
                        line-height: 21px;
                        color: #848484;
                    }
                }

                &__button {
                    font-family: 'font_regular', sans-serif;
                    font-weight: 700;
                    margin-top: 40px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    background-color: #376fff;
                    border-radius: 50px;
                    color: #fff;
                    cursor: pointer;
                    width: 100%;
                    height: 48px;

                    &:hover {
                        background-color: #0059d0;
                    }
                }

                &__cancel {
                    align-self: center;
                    font-size: 16px;
                    line-height: 21px;
                    color: #0068dc;
                    text-align: center;
                    margin-top: 30px;
                    cursor: pointer;
                }

                &__recovery {
                    font-size: 16px;
                    line-height: 19px;
                    color: #0068dc;
                    cursor: pointer;
                    margin-top: 20px;
                    text-align: center;
                    width: 100%;
                }
            }

            &__footer-item {
                margin-top: 30px;
                font-size: 14px;
            }
        }
    }

    .logo {
        cursor: pointer;
        width: 207px;
        height: 37px;
    }

    .disabled,
    .disabled-button {
        pointer-events: none;
        color: #acb0bc;
    }

    .disabled-button {
        background-color: #dadde5;
        border-color: #dadde5;
    }

    @media screen and (width <= 750px) {

        .authorize-area {

            &__content-area {

                &__container {
                    width: 100%;
                    padding: 60px;
                }
            }

            &__expand {

                &__dropdown {
                    left: -200px;
                }
            }
        }
    }

    @media screen and (width <= 414px) {

        .authorize-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 0 20px 20px;
                    background: transparent;
                }
            }
        }
    }
</style>
