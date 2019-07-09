// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="search-container">
        <div class="search-container__wrap">
            <label class="search-container__wrap__input">
                <input v-model="search" v-on:input="fetch" placeholder="Search Buckets" type="text">
            </label>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { BUCKET_USAGE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        methods: {
            fetch: async function() {
                const bucketsResponse = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1);
                if (!bucketsResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + bucketsResponse.errorMessage);
                }
            }
        },
        computed: {
            search: {
                get: function (): string {
                    return this.$store.state.bucketUsageModule.cursor.search;
                },
                set: function (search: string) {
                    this.$store.dispatch(BUCKET_USAGE_ACTIONS.SET_SEARCH, search)
                }
            }
        }
    })

    export default class SearchArea extends Vue {}
</script>

<style scoped lang="scss">
    .search-container {
        width: 100%;
        height: 56px;
        margin: 0 24px;
        
        &__wrap {
            position: relative;
            
            &::after {
                content: '';
                display: block;
                position: absolute;
                height: 20px;
                top: 50%;
                transform: translateY(-50%);
                right: 20px;
                width: 20px;
                background-image: url('../../../static/images/team/searchIcon.svg');
                background-repeat: no-repeat;
                background-size: cover;
                z-index: 20;
            }
            
            &__input {
                
                input {
                    box-sizing: border-box;
                    position: relative;
                    box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
                    border-radius: 6px;
                    border: 1px solid #F2F2F2;
                    width: 100%;
                    height: 56px;
                    padding-right: 20px;
                    padding-left: 20px;
                    font-family: 'font_regular';
                    font-size: 16px;
                    outline: none;
                    box-shadow: none;
                }
            }
        }
    }
    
    ::-webkit-input-placeholder {
        color: #AFB7C1;
    }
</style>
