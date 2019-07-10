// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <h1>Account</h1>
        <TabNavigation
            class="account-area-container__navigation"
            :navigation="navigation"/>
        <router-view />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import TabNavigation from '@/components/navigation/TabNavigation.vue';
    import { ACCOUNT_ROUTES } from '@/utils/constants/tabNavigation';

    @Component({
    	mounted() {
    		this.$router.push(ACCOUNT_ROUTES.PROFILE.path);
    	},
    	data: function () {
    		return {
    			navigation: ACCOUNT_ROUTES,
    		};
    	},
    	components: {
    		TabNavigation,
    	},
    })

    export default class AccountArea extends Vue {}
</script>

<style scoped lang="scss">
    .account-area-container {
        padding: 44px 55px 55px 55px;
        position: relative;
        
        &__navigation {
            position: absolute;
            right: 55px;
            top: 44px;
            z-index: 99;
            background-color: #F5F6FA;
        }
        
        h1 {
            position: absolute;
            left: 55px;
            top: 44px;
            z-index: 99;
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
    }
</style>
