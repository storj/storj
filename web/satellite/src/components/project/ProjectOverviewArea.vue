// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-overview">
        <ProjectNavigation class="project-overview__navigation" v-if="isProjectSelected"/>
        <router-view v-if="isProjectSelected"/>
        <EmptyState
            v-if="!isProjectSelected"
            mainTitle="Create your first project"
            additional-text='<p>Please click the button <span style="font-family: font_bold">"New Project"</span> in the right corner</p>'
            :imageSource="emptyImage" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import ProjectNavigation from '@/components/project/ProjectNavigation.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component(
    {
        data: function () {
            return {
                emptyImage: EMPTY_STATE_IMAGES.PROJECT,
            };
        },
        computed: {
            isProjectSelected: function (): boolean {
                return this.$store.getters.selectedProject.id !== '';
            },
        },
        components: {
            EmptyState,
            ProjectNavigation,
        }
    }
)

export default class ProjectDetailsArea extends Vue {
}
</script>

<style scoped lang="scss">
    .project-overview {
        padding: 44px 55px 55px 55px;
        position: relative;
        overflow-x: hidden;
        height: 85vh;

        &__navigation {
            position: absolute;
            right: 55px;
            top: 44px;
            z-index: 1000;
        }
    }
</style>
