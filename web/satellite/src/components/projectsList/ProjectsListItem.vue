// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <p class="container__item name" :title="itemData.name">{{ itemData.name }}</p>
        <p class="container__item">{{ itemData.description }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Project } from '@/types/projects';

@Component
export default class ProjectsListItem extends Vue {
    @Prop({default: () => new Project('123', 'name', 'desc')})
    private readonly itemData: Project;

    public mounted() {
        console.log("LISTITEM:", this.itemData)
    }
}
</script>

<style scoped lang="scss">
    .container {
        padding: 20px 40px;
        outline: none;
        display: flex;
        background: #fff;
        margin-bottom: 1px;
        width: calc(100% - 80px);

        &__item {
            width: 20%;
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin: 0;
        }
    }

    .name {
        width: 40%;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
</style>
