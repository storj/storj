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
            additional-text="<p>We recommend you to create your first API Key for this project. API Keys allow developers to manage their project and build applications over Storj Network through our Uplink CLI.</p>"
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
import Footer from '@/components/apiKeys/footerArea/Footer.vue';

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
        padding: 55px 30px 0px 64px;
        max-width: 79.7%;
        width: 100%;
        background-color: #F5F6FA;
        z-index: 999;
    }
    .api-keys-container {
       padding: 0px 30px 55px 64px;
       overflow-y: scroll;
       max-height: 84vh;
       position: relative;

       &__content {
            display: grid;
            grid-template-columns: 190px 190px 190px 190px 190px 190px 190px;
            width: 100%;
            grid-row-gap: 20px;
            grid-column-gap: 20px;
            justify-content: space-between;
            margin-top: 150px;
            margin-bottom: 100px;
        }
   }

    @media screen and (max-width: 1600px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 200px 200px 200px 200px 200px;
            }
        }
        .api-keys-header {
            max-width: 75%;
        }
    }

    @media screen and (max-width: 1366px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 180px 180px 180px 180px 180px;
            }
        }

        .api-keys-header {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1281px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 210px 210px 210px 210px;
            }
        }

        .api-keys-header {
            max-width: 69%;
        }

        .apikey-item-container {
            height: 200px;
        }
    }

    @media screen and (max-width: 1025px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 200px 200px 200px 200px;
            }
        }
        .api-keys-header {
            max-width: 80%;
        }

        .apikey-item-container {
            height: 180px;
        }
    }

    @media screen and (max-width: 801px) {
        .api-keys-container {

            &__content {
                grid-template-columns: 200px 200px 200px;
                grid-row-gap: 0px;
            }
        }
        .api-keys-header {
            max-width: 80%;
        }

        .apikey-item-container {
            height: 200px;
        }
    }
</style>