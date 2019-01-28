// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="apiKeyItems" class="api-keys-header">
            <HeaderArea/>
        </div>
        <div v-if="apiKeyItems" class="api-keys-container">
            <div class="api-keys-container__content">
                <ApiKeysItem class="selected"/>
                <ApiKeysItem />
                <ApiKeysItem />
            </div>
            <Footer/>
        </div>
        <EmptyState
            v-if="!apiKeyItems"
            mainTitle="You have no API Keys yet"
            :imageSource="emptyImage"
            buttonLabel="New Api Key"
            isButtonShown />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import HeaderArea from '@/components/apiKeys/headerArea/HeaderArea.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import Footer from '@/components/apiKeys/footerArea/Footer.vue'

@Component({
    data: function () {
        return {
            emptyImage: EMPTY_STATE_IMAGES.API_KEY,
            apiKeyItems: true,
        };
    },
    methods: {},
    computed: {},
    components: {
        EmptyState,
        HeaderArea,
        ApiKeysItem,
        Footer
    }
})

export default class ApiKeysArea extends Vue {
}
</script>

<style scoped lang="scss">
    // .api-keys-header {
    //     padding: 44px 40px 0 40px;
    // }

    // .api-keys-container {
    //     padding: 0px 40px 0 60px;
    // }
    .api-keys-header {
        position: fixed;
        padding: 55px 30px 25px 64px;
        max-width: 79.7%;
        width: 100%;
        background-color: rgba(255,255,255,0.6);
        z-index: 999;
    }
    .api-keys-container {
       padding: 0px 30px 55px 64px;
       overflow-y: scroll;
       max-height: 84vh;
       position: relative;

       &__content {
            display: grid;
            grid-template-columns: 190px 190px 190px 190px 190px 190px ;
            width: 100%;
            grid-row-gap: 20px;
            justify-content: space-between;
            margin-top: 175px;
            margin-bottom: 100px;
        }
   }

    @media screen and (max-width: 1600px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px 190px 190px;
            }
        }
        .api-keys-header {
            max-width: 73.7%;
        }
    }

    @media screen and (max-width: 1366px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px 190px;
            }
        }

        .api-keys-header {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1120px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 190px 190px 190px;
                grid-row-gap: 0px;
            }
        }
        .api-keys-header {
            max-width: 82.7%;
        }
    }
</style>