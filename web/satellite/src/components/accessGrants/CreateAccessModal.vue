// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-access">
        <div class="create-access__header-container">
            <h2 class="create-access__header-container__title">Create Access</h2>
            <div
            class="create-grant__header-container__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon />
            </div>
        </div>
        <div class="create-access__body-container">

        </div>
        <div class="create-access__footer-container">
            <VButton
                label="Learn More"
                width="auto"
                height="30px"
                is-transparent="true"
                font-size="13px"
                class="create-access__footer-container__learn-button"
            />
            <VButton
                label="Encrypt My Access"
                font-size="13px"
                width="auto"
                height="30px"
                class="create-access__footer-container__encrypt-button"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import CheckboxIcon from '@/../static/images/accessGrants/vector.svg';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';

// @vue/component
@Component({
    components: {
        CloseCrossIcon,
        CheckboxIcon,
    },
})
export default class CreateAccessGrant extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    /**
     * Closes modal.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.$router.push(RouteConfig.AccessGrants.path);
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

    .create-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgba(27, 37, 51, 0.75);
        display: flex;
        align-items: center;
        justify-content: center;

        &__container {
            background: #f5f6fa;
            border-radius: 6px;
            display: flex;
            align-items: flex-start;
            position: relative;

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

    @media screen and (max-height: 800px) {

        .create-grant {
            padding: 50px 0 20px 0;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 750px) {

        .create-grant {
            padding: 100px 0 20px 0;
        }
    }

    @media screen and (max-height: 700px) {

        .create-grant {
            padding: 150px 0 20px 0;
        }
    }

    @media screen and (max-height: 650px) {

        .create-grant {
            padding: 200px 0 20px 0;
        }
    }

    @media screen and (max-height: 600px) {

        .create-grant {
            padding: 250px 0 20px 0;
        }
    }

    @media screen and (max-height: 550px) {

        .create-grant {
            padding: 300px 0 20px 0;
        }
    }

    @media screen and (max-height: 500px) {

        .create-grant {
            padding: 350px 0 20px 0;
        }
    }
</style>
