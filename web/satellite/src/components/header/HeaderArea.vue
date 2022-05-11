// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__left-area">
            <div class="header-container__left-area__logo-area">
                <NavigationMenuIcon
                    v-if="!isNavigationVisible && !isOnboardingTour"
                    id="navigation-menu-icon"
                    class="header-container__left-area__logo-area__menu-button"
                    @click.stop="toggleNavigationVisibility"
                />
                <div v-if="isNavigationVisible" class="header-container__left-area__logo-area__close-button" @click.stop="toggleNavigationVisibility">
                    <NavigationCloseIcon />
                    <p class="header-container__left-area__logo-area__close-button__title">Close</p>
                </div>
                <LogoIcon
                    class="logo"
                    @click.stop="onLogoClick"
                />
            </div>
            <div class="header-container__left-area__selection-wrapper">
                <ProjectSelection class="project-selection" />
                <ResourcesSelection class="resources-selection" />
                <SettingsSelection class="settings-selection" />
            </div>
        </div>
        <div class="header-container__right-area">
            <VInfo
                v-if="!isOnboardingTour"
                class="header-container__right-area__info"
                title="Need some help?"
                button-label="Start tour"
                :on-button-click="onStartTourButtonClick"
            >
                <template #icon>
                    <InfoIcon class="header-container__right-area__info__icon" aria-roledescription="restart-onb-icon" />
                </template>
                <template #message>
                    <p class="header-container__right-area__info__message">
                        You can always start the onboarding tour and go through all the steps to get you started again.
                    </p>
                </template>
            </VInfo>
            <div class="header-container__right-area__satellite-area">
                <div class="header-container__right-area__satellite-area__checkmark">
                    <CheckmarkIcon />
                </div>
                <p class="header-container__right-area__satellite-area__name">{{ satelliteName }}</p>
            </div>
            <AccountButton class="header-container__right-area__account-button" />
        </div>
        <NavigationArea
            v-if="isNavigationVisible"
            class="adapted-navigation"
        />
        <div v-if="isNavigationVisible" class="navigation-blur" @click.self.stop="toggleNavigationVisibility" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelection from '@/components/header/projectsDropdown/ProjectSelection.vue';
import ResourcesSelection from '@/components/header/resourcesDropdown/ResourcesSelection.vue';
import SettingsSelection from '@/components/header/settingsDropdown/SettingsSelection.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import VInfo from '@/components/common/VInfo.vue';

import LogoIcon from '@/../static/images/logo.svg';
import InfoIcon from '@/../static/images/header/info.svg';
import CheckmarkIcon from '@/../static/images/header/checkmark.svg';
import NavigationCloseIcon from '@/../static/images/header/navigationClose.svg';
import NavigationMenuIcon from '@/../static/images/header/navigationMenu.svg';

import { RouteConfig } from '@/router';

import AccountButton from './accountDropdown/AccountButton.vue';

// @vue/component
@Component({
    components: {
        AccountButton,
        NavigationArea,
        VInfo,
        NavigationMenuIcon,
        NavigationCloseIcon,
        LogoIcon,
        InfoIcon,
        CheckmarkIcon,
        ProjectSelection,
        ResourcesSelection,
        SettingsSelection,
    },
})
export default class HeaderArea extends Vue {
    /**
     * Indicates if navigation toggling button is visible depending on screen width.
     */
    public isNavigationVisible = false;

    /**
     * Toggle navigation visibility.
     */
    public toggleNavigationVisibility(): void {
        this.isNavigationVisible = !this.isNavigationVisible;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Redirects to onboarding tour.
     */
    public async onStartTourButtonClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.path).catch(() => {return; })
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Returns satellite name from config.
     */
    public get satelliteName(): string {
        return this.$store.state.appStateModule.satelliteName;
    }
}
</script>

<style scoped lang="scss">
    .header-container {
        font-family: 'font_regular', sans-serif;
        min-height: 62px;
        background-color: #fff;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 30px 0 10px;
        position: relative;

        &__left-area {
            display: flex;
            align-items: center;

            &__logo-area {
                width: 220px;
                margin-right: 20px;

                &__menu-button {
                    display: none;
                }

                &__close-button {
                    display: flex;
                    align-items: center;
                    cursor: pointer;

                    &__title {
                        margin-left: 13px;
                        font-size: 16px;
                        line-height: 23px;
                        color: #a9b5c1;
                    }
                }
            }

            &__selection-wrapper {
                display: flex;
            }
        }

        &__right-area {
            display: flex;
            align-items: center;

            &__info {
                max-height: 24px;
                margin-right: 17px;

                &__icon {
                    cursor: pointer;
                }

                &__message {
                    color: #586c86;
                    font-family: 'font_regular', sans-serif;
                    font-size: 12px;
                    line-height: 21px;
                }
            }

            &__satellite-area {
                height: 36px;
                background: #f6f6fa;
                border-radius: 58px;
                display: flex;
                align-items: center;
                padding: 0 10px 0 5px;
                margin-right: 20px;

                &__checkmark {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    width: 22px;
                    height: 22px;
                    background: #fff;
                    border-radius: 11px;
                }

                &__name {
                    font-weight: 500;
                    font-size: 14px;
                    line-height: 18px;
                    color: #000;
                }
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .adapted-navigation {
        position: absolute;
        top: 62px;
        left: 0;
        z-index: 100;
        height: 100vh;
        background: #e6e9ef;
    }

    .navigation-blur {
        height: 100vh;
        width: 100%;
        position: absolute;
        top: 62px;
        left: 220px;
        z-index: 99;
        background: rgb(134 134 148 / 40%);
    }

    .project-selection,
    .resources-selection {
        margin-right: 35px;
    }

    ::v-deep .info__box {
        top: 100%;

        &__message {
            min-width: 335px;
        }
    }

    @media screen and (min-width: 1281px) {

        .adapted-navigation,
        .navigation-blur,
        .header-container__left-area__logo-area__close-button {
            display: none;
        }
    }

    @media screen and (max-width: 1280px) {

        .header-container {
            padding: 0 40px;

            &__left-area {

                &__logo-area {

                    &__menu-button {
                        display: block;
                        cursor: pointer;
                    }
                }
            }
        }

        .logo {
            display: none;
        }

        ::v-deep .edit-project {
            margin-left: 0;
        }
    }

    @media screen and (max-width: 1024px) {

        .header-container {

            &__left-area {

                &__logo-area {
                    width: 100px;
                }
            }
        }

        .project-selection,
        .resources-selection {
            margin-right: 15px;
        }
    }

    @media screen and (max-width: 768px) {

        .header-container {

            &__left-area {

                &__selection-wrapper {
                    display: none;
                }
            }
        }
    }

</style>
