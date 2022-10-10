// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="confirm-delete">
        <div class="confirm-delete__container">
            <h1 class="confirm-delete__container__title">Delete Access</h1>
            <p class="confirm-delete__container__info">
                You won’t be able to access bucket(s) or object(s) related to the access:
            </p>
            <div class="confirm-delete__container__list">
                <div
                    v-for="accessGrant in selectedAccessGrants"
                    :key="accessGrant.id"
                    class="confirm-delete__container__list__container"
                >
                    <div class="confirm-delete__container__list__container__item">
                        <p class="confirm-delete__container__list__container__item__name">
                            {{ accessGrant.name }}
                        </p>
                    </div>
                </div>
            </div>
            <div>
                <p>This action cannot be undone.</p>
            </div>
            <VInput
                placeholder="Type the name of the access"
                @setData="setConfirmedInput"
            />
            <div class="confirm-delete__container__buttons-area">
                <VButton
                    class="cancel-button"
                    label="Cancel"
                    width="70px"
                    height="44px"
                    :on-press="onCancelClick"
                    is-white="true"
                    :is-disabled="isLoading"
                />
                <VButton
                    label="Delete Access"
                    width="150px"
                    height="44px"
                    :on-press="onDeleteClick"
                    :is-disabled="isLoading || confirmedInput !== selectedAccessGrants[0].name"
                    is-solid-delete="true"
                    has-trash-icon="true"
                />
            </div>
            <div class="confirm-delete__container__close-cross-container" @click="onCancelClick">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant } from '@/types/accessGrants';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VInput,
        CloseCrossIcon,
    },
})
export default class ConfirmDeletePopup extends Vue {
    private readonly FIRST_PAGE: number = 1;
    private isLoading = false;
    private confirmedInput = '';

    /**
     * sets Comfirmed Input property to the given value.
     */
    public setConfirmedInput(value: string): void {
        this.confirmedInput = value;
    }

    /**
     * Deletes selected access grants, fetches updated list and closes popup.
     */
    public async onDeleteClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE);
            await this.$notify.success(`Access Grant deleted successfully`);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);
        }

        this.$emit('resetPagination');
        this.isLoading = false;
        this.onCancelClick();
    }

    /**
     * Closes popup
     */
    public onCancelClick(): void {
        this.$emit('close');
    }

    /**
     * Returns list of selected access grants from store.
     */
    public get selectedAccessGrants(): AccessGrant[] {
        return this.$store.getters.selectedAccessGrants;
    }
}
</script>

<style scoped lang="scss">
    .confirm-delete {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;
        font-style: normal;

        &__trash-icon {
            position: absolute;
            left: 57%;
            margin-top: -3px;
        }

        &__text-container {
            text-align: left;
        }

        &__container {
            border-radius: 6px;
            max-width: 325px;
            padding: 40px 30px;
            position: relative;
            display: flex;
            flex-direction: column;
            background-color: #fff;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-weight: bold;
                font-size: 28px;
                line-height: 34px;
                color: #000;
            }

            &__info {
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                color: #000;
                margin: 25px 0 10px;
            }

            &__info-new {
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                text-align: left;
                color: #000;
                margin: 20px 0;
            }

            &__list-label {
                font-weight: bold;
                font-size: 14px;
                line-height: 18px;
                color: #e30011;
                font-family: 'font_medium', sans-serif;
                white-space: nowrap;
                margin-bottom: 30px;
            }

            &__list {
                max-height: 255px;
                overflow-y: scroll;
                border-radius: 6px;
                width: 100%;

                &__container {

                    &__item {
                        padding: 3px 7px;
                        max-width: fit-content;
                        background: #d8dee3;
                        border-radius: 20px;
                        margin-bottom: 10px;

                        &__name {
                            font-family: 'font_medium', sans-serif;
                            margin: 0;
                            font-weight: bold;
                            font-size: 17px;
                            line-height: 30px;
                            color: #1b2533;
                            overflow: hidden;
                            text-overflow: ellipsis;
                            white-space: nowrap;
                        }
                    }
                }
            }

            &__buttons-area {
                width: fit-content;
                display: flex;
                align-items: center;
                margin-top: 30px;
            }

            &__close-cross-container {
                display: flex;
                justify-content: center;
                align-items: center;
                position: absolute;
                right: 30px;
                top: 30px;
                height: 24px;
                width: 24px;
                cursor: pointer;

                &:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }
        }
    }

    .cancel-button {
        margin-right: 15px;
    }
</style>