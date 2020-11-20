// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generate-grant">
        <BackIcon class="generate-grant__back-icon" @click="onBackClick"/>
        <h1 class="generate-grant__title">Generate Access Grant</h1>
        <div class="generate-grant__warning">
            <div class="generate-grant__warning__header">
                <WarningIcon/>
                <h2 class="generate-grant__warning__header__label">This Information is Only Displayed Once</h2>
            </div>
            <p class="generate-grant__warning__message">
                Save this information in a password manager, or wherever you prefer to store sensitive information.
            </p>
        </div>
        <div class="generate-grant__grant-area">
            <h3 class="generate-grant__grant-area__label">Access Grant</h3>
            <div class="generate-grant__grant-area__container">
                <p class="generate-grant__grant-area__container__value">{{ access }}</p>
                <VButton
                    class="generate-grant__grant-area__container__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    :on-press="onCopyClick"
                />
            </div>
        </div>
        <div class="generate-grant__gateway-area">
            <div class="generate-grant__gateway-area__toggle" @click="toggleCredentialsVisibility">
                <h3 class="generate-grant__gateway-area__toggle__label">Gateway Credentials</h3>
                <ExpandIcon v-if="!areGatewayCredentialsVisible"/>
                <HideIcon v-else/>
            </div>
            <div class="generate-grant__gateway-area__container" v-if="areGatewayCredentialsVisible">
                <h3 class="generate-grant__gateway-area__container__title">
                    Using Gateway Credentials Enables Server-Side Encryption.
                </h3>
                <p class="generate-grant__gateway-area__container__disclaimer">
                    By generating gateway credentials, you are opting in to Server-side encryption
                </p>
                <VButton
                    class="generate-grant__gateway-area__container__button"
                    label="Generate Gateway Credentials"
                    width="100%"
                    height="48px"
                    :on-press="onGenerateCredentialsClick"
                    is-disabled="true"
                />
            </div>
        </div>
        <VButton
            class="generate-grant__done-button"
            label="Done"
            width="100%"
            height="48px"
            :on-press="onDoneClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import WarningIcon from '@/../static/images/accessGrants/warning.svg';
import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';
import HideIcon from '@/../static/images/common/BlackArrowHide.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        BackIcon,
        WarningIcon,
        VButton,
        ExpandIcon,
        HideIcon,
    },
})
export default class ResultStep extends Vue {
    public access: string = '';
    public areGatewayCredentialsVisible: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Sets local access from props value.
     */
    public mounted(): void {
        if (!this.$route.params.access || !this.$route.params.key) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.access = this.$route.params.access;
    }

    /**
     * Toggles gateway credentials section visibility.
     */
    public toggleCredentialsVisibility(): void {
        this.areGatewayCredentialsVisible = !this.areGatewayCredentialsVisible;
    }

    /**
     * Holds on copy button click logic.
     * Copies token to clipboard.
     */
    public onCopyClick(): void {
        this.$copyText(this.access);
        this.$notify.success('Token was copied successfully');
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        if (this.accessGrantsAmount > 1) {
            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).name,
                params: {
                    key: this.$route.params.key,
                },
            });

            return;
        }

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).name,
            params: {
                key: this.$route.params.key,
            },
        });
    }

    /**
     * Holds on done button click logic.
     * Proceed to upload data step.
     */
    public onDoneClick(): void {
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.UploadStep)).name,
            params: {
                isUplinkSectionEnabled: 'false',
            },
        });
    }

    /**
     * Holds on generate credentials button click logic.
     * Generates gateway credentials.
     */
    public onGenerateCredentialsClick(): void {
        return;
    }

    /**
     * Returns amount of access grants from store.
     */
    private get accessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.page.accessGrants.length;
    }
}
</script>

<style scoped lang="scss">
    .generate-grant {
        height: calc(100% - 60px);
        padding: 30px 65px;
        max-width: 475px;
        min-width: 475px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 30px 0;
        }

        &__warning {
            padding: 20px;
            width: calc(100% - 40px);
            background: #fff9f7;
            border: 1px solid #f84b00;
            border-radius: 8px;

            &__header {
                display: flex;
                align-items: center;

                &__label {
                    font-style: normal;
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0 0 0 15px;
                }
            }

            &__message {
                font-size: 16px;
                line-height: 22px;
                color: #1b2533;
                margin: 8px 0 0 0;
            }
        }

        &__grant-area {
            margin: 40px 0;
            width: 100%;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 0 0 10px 0;
            }

            &__container {
                display: flex;
                align-items: center;
                border-radius: 9px;
                padding: 10px;
                width: calc(100% - 20px);
                border: 1px solid rgba(56, 75, 101, 0.4);

                &__value {
                    text-overflow: ellipsis;
                    overflow: hidden;
                    white-space: nowrap;
                    margin: 0;
                }

                &__button {
                    min-width: 66px;
                    min-height: 30px;
                    margin-left: 10px;
                }
            }
        }

        &__gateway-area {
            display: flex;
            flex-direction: column;
            align-items: center;

            &__toggle {
                display: flex;
                align-items: center;
                cursor: pointer;

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #49515c;
                    margin: 0 15px 0 0;
                }
            }

            &__container {
                background: #f5f6fa;
                border-radius: 6px;
                padding: 40px 50px;
                width: calc(100% - 100px);
                margin-top: 30px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 18px;
                    line-height: 24px;
                    color: #000;
                    margin: 0 0 20px 0;
                    text-align: center;
                }

                &__disclaimer {
                    font-size: 16px;
                    line-height: 28px;
                    color: #000;
                    margin: 0 0 25px 0;
                    text-align: center;
                }
            }
        }

        &__done-button {
            margin-top: 30px;
        }
    }
</style>
