// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="empty-state">
        <div class="empty-state__modal">
            <Key />
            <h4 class="empty-state__modal__heading">Create Your First Access Grant</h4>
            <p class="empty-state__modal__subheading">Get started by creating an Access to interact with your Buckets</p>
            <VButton
                label="Create Access Grant +"
                width="199px"
                height="44px"
                class="empty-state__modal__cta"
                :on-press="onCreateClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';

import Key from '@/../static/images/accessGrants/key.svg';

// @vue/component
@Component({
    components: {
        Key,
        VButton,
    },
})
export default class EmptyState extends Vue {

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
    
    /**
     * Starts create access grant flow.
     */
    public onCreateClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
    }
}
</script>

<style scoped lang="scss">
    .empty-state {
        background-image: url('../../../static/images/accessGrants/access-grants-bg.png');
        background-size: contain;
        margin-top: 40px;

        &__modal {
            display: block;
            margin-left: auto;
            margin-right: auto;
            width: 660px;
            text-align: center;
            background: #fff;
            padding: 100px 30px;
            position: relative;
            top: 110px;

            &__heading {
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                font-weight: 700;
                line-height: 16px;
                margin-bottom: 30px;
            }

            &__subheading {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                font-weight: 400;
                line-height: 21px;
            }

            &__cta {
                margin: 25px auto 0;
            }
        }
    }
</style>
