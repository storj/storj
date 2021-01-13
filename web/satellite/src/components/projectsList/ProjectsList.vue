// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-list">
        <div class="projects-list__title-area">
            <h2 class="projects-list__title-area__title">Projects</h2>
        </div>
    </div>
</template>

<script>
import { Component, Vue } from 'vue-property-decorator';

@Component({
    components:{}
})

export default class Projects extends Vue {

}
</script>

<style lang="scss">
    .projects-list {
        position: relative;
        padding: 40px 30px 55px 30px;
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #263549;
                margin: 0;
            }
        }
    }
</style>
