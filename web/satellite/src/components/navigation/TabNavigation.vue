// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tab-navigation-container">
        <router-link :to="navLink.path" class="tab-navigation-container__item" v-for="navLink in navigation" :key="navLink.name">
            <p>{{navLink.name}}</p>
        </router-link>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component({})
    export default class TabNavigation extends Vue {
        // TODO: add types for navigation
        @Prop({default: {}})
        private navigation: any;
    }
</script>

<style scoped lang="scss">
    .tab-navigation-container {
        width: auto;
        display: flex;
        border-bottom: 1px solid #AFB7C1;
        
        &__item {
            width: 150px;
            height: 48px;
            display: flex;
            align-items: center;
            justify-content: center;
            
            p {
                font-family: 'font_medium';
                font-size: 14px;
                line-height: 20px;
                color: #AFB7C1;
            }
            
            &.router-link-exact-active,
            &:hover {
                border-bottom: 3px solid #2683FF;
                
                p {
                    color: #354049;
                }
            }
        }
    }
</style>
