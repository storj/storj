// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-project-referral-container">
        <img
            class="no-project-referral-container__illustration"
            src="../../../../static/images/referral/Rocket.svg"
            alt="Rocket">
        <h1 class="no-project-referral-container__title">It’s Time to Get Started</h1>
        <h2 class="no-project-referral-container__subtitle">Let’s get started and create your first distributed storage
            project.</h2>
        <Button
            class="no-project-referral-container__create-button"
            label="+ Create Project"
            height="56px"
            width="170px"
            :onPress="onCreateProjectClick"/>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            Button,
        }
    })

    export default class NoProjectStateArea extends Vue {

        public onCreateProjectClick() {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
        }
    }
</script>

<style scoped lang="scss">
    h1,
    h2 {
        margin-block-start: 0;
        margin-block-end: 0;
    }

    .no-project-referral-container {
        padding: 80px 40px;
        background-color: #fff;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        border-radius: 24px;

        &__title {
            margin-top: 40px;
            font-family: 'font_bold';
            font-size: 34px;
            line-height: 49px;
            color: #494949;
        }

        &__subtitle {
            margin-top: 40px;
            font-family: 'font_regular';
            font-size: 18px;
            line-height: 28px;
            color: #747474;
        }

        &__create-button {
            margin-top: 42px;
            min-height: 56px;
        }
    }
</style>
