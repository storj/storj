// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="container">
        <td class="container__item">{{ bucket.bucketName }}</td>
        <td class="container__item">{{ storage }}</td>
        <td class="container__item">{{ egress }}</td>
        <td class="container__item">{{ objectCount }}</td>
    </tr>
</template>

<script lang="ts">
    import { Component, Vue, Prop } from 'vue-property-decorator';

    // TODO: should it be functional?
    @Component
    export default class BucketItem extends Vue {
        @Prop()
        private readonly bucket: BucketUsage;
    
        public get storage(): string {
            return this.bucket.storage.toFixed(4);
        }
    
        public get egress(): string {
            return this.bucket.egress.toFixed(4);
        }
        
        public get objectCount(): string {
            return this.bucket.objectCount.toString();
        }
    }
</script>

<style scoped lang="scss">
    .container {
        transition: box-shadow .2s ease-out;
        padding: 35px 0px 35px 70px;
        transition: all .2s ease;
        margin-bottom: 10px;
        border-radius: 6px;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        outline: none;
        
        &:hover {
            background: #fff;
            box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
        }
        
        &__item {
            width: 20%;
            height: 80px;
            padding-left: 30px;
            font-family: 'font_medium';
            font-size: 16px;
            margin: 0;
        }
        
        &:first-of-type {
            font-weight: bold;
        }
    }
    
    @media screen and (max-width: 1600px) {
        .container {
            grid-template-columns: 2% 20% 30% 20% 15% 13%;
            padding: 20px 0px 20px 70px;
        }
    }
</style>
