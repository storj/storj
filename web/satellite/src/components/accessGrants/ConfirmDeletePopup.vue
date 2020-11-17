// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="confirm-delete">
        <div class="confirm-delete__container">
            <h1 class="confirm-delete__container__title">Are you sure?</h1>
            <p class="confirm-delete__container__info">
                When an access grant is removed, users using it will no longer have access to the buckets or dat.
            </p>
            <p class="confirm-delete__container__list-label">
                The following access grant(s) will be removed from this project:
            </p>
            <div class="confirm-delete__container__list">
                <div
                    class="confirm-delete__container__list__container"
                    v-for="accessGrant in selectedAccessGrants"
                    :key="accessGrant.id"
                >
                    <div class="confirm-delete__container__list__container__item">
                        <p class="confirm-delete__container__list__container__item__name">
                            {{accessGrant.name}}
                        </p>
                    </div>
                </div>
            </div>
            <div class="confirm-delete__container__buttons-area">
                <VButton
                    class="cancel-button"
                    label="Cancel"
                    width="50%"
                    height="44px"
                    :on-press="onCancelClick"
                    is-white="true"
                />
                <VButton
                    label="Remove"
                    width="50%"
                    height="44px"
                    :on-press="onDeleteClick"
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

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant } from '@/types/accessGrants';

@Component({
    components: {
        VButton,
        CloseCrossIcon,
    },
})
export default class ConfirmDeletePopup extends Vue {
    private FIRST_PAGE: number = 1;

    /**
     * Deletes selected access grants, fetches updated list and closes popup.
     */
    public async onDeleteClick(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE);
            await this.$notify.success(`Access Grant deleted successfully`);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);
        }

        this.$emit('reset-pagination');
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
        background: rgba(27, 37, 51, 0.75);
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;
        font-style: normal;

        &__container {
            border-radius: 6px;
            max-width: 475px;
            padding: 50px 65px;
            position: relative;
            display: flex;
            flex-direction: column;
            align-items: center;
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
                text-align: center;
                color: #000;
            }

            &__list-label {
                font-weight: bold;
                font-size: 14px;
                line-height: 18px;
                color: #e30011;
                font-family: 'font_medium', sans-serif;
            }

            &__list {
                max-height: 255px;
                overflow-y: scroll;
                border-radius: 6px;
                width: 100%;

                &__container {

                    &__item {
                        padding: 25px;
                        width: calc(100% - 50px);
                        background: rgba(245, 246, 250, 0.6);

                        &__name {
                            font-family: 'font_medium', sans-serif;
                            margin: 0;
                            font-weight: bold;
                            font-size: 14px;
                            line-height: 19px;
                            color: #1b2533;
                            text-overflow: ellipsis;
                            white-space: nowrap;
                        }
                    }
                }
            }

            &__buttons-area {
                width: 100%;
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