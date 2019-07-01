// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="referral-container">
		<NoProjectState v-if="isNoProjects"/>
		<div class="referral-container__wrapper" v-if="!isNoProjects">
			<p class="referral-container__title">Refer A Friend And You'll Both Get Free Credits</p>
			<p class="referral-container__subtitle">You Get X GB, They Get X GB</p>
			<div class="referral-container__icons">
                <div class="icon-container">
				    <img src="../../../../static/images/referral/send.svg">
                    <p class="referral-container__icons__description">Copy and share your invite link</p>
                </div>
                <div class="icon-container">
                    <img src="../../../../static/images/referral/coins.svg">
				    <p class="referral-container__icons__description">They get XGB in free credits</p>
                </div>
                <div class="icon-container">
                    <img src="../../../../static/images/referral/calendar.svg">
				    <p class="referral-container__icons__description">They use our network for 60 days</p>  
                </div>
                <div class="icon-container">
                    <img src="../../../../static/images/referral/money.svg">
				    <p class="referral-container__icons__description">You get XGb in free credits</p>
                </div>
			</div>
			<div class="referral-container__copy-and-share-container">
				<p class="referral-container__copy-and-share-container__title">Copy And Share Your Invitation Link</p>
				<div class="referral-container__copy-and-share-container__link-holder">
					<p class="referral-container__copy-and-share-container__link-holder__link">test://test/test/test/test</p>
					<div class="copy-button">Copy</div>
				</div>
			</div>
            <div class="underline"></div>
			<ReferralStats />
			<p class="referral-container__help">Referral credits are only earned after your referral has used up their free credits. Credits will automatically be applied to your upcoming bill. There are unlimited invites, so invite as many people as you want. The invite you send is only for the Satellite you're connected to. If you have any questions please email 
                <a href="mailto:support@tardigrade.io">support@tardigrade.io</a>
            </p>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import ReferralStats from './ReferralStats.vue';
    import NoProjectState from './NoProjectStateArea.vue';

    @Component({
        computed: {
            isNoProjects: function (): boolean {
                return !this.$store.state.projectsModule.projects.length;
            }
        },
        components: {
            ReferralStats,
            NoProjectState,
        }
    })

    export default class ReferralArea extends Vue {}
</script>

<style scoped lang="scss">
	p {
		margin: 0;
	}

	::-webkit-scrollbar {
		width: 0;
	}

	.referral-container {
		position: relative;
		margin-top: 83px;
		font-family: font_regular;
		display: flex;
		flex-direction: column;
		justify-items: center;
		align-items: center;

		&__wrapper {
			width: 966px;
			display: flex;
			justify-content: flex-start;
			flex-direction: column;
		}

		&__title {
			text-align: center;
			font-size: 32px;
			line-height: 33px;
			color: #354049;
			font-family: font_bold;
            margin-top: 80px;
		}

		&__subtitle {
			text-align: center;
			font-size: 22px;
			line-height: 24px;
            margin-top: 25px;
			color: #354049;
		}

		&__icons {
			display: flex;
			justify-content: space-between;
            align-items: center;
            margin-top: 56px;
            min-height: 123px;
			padding: 0 20px 0 20px;

            &__description {
				max-width: 220px;
				text-align: center;
				font-size: 18px;
				line-height: 27px;
                margin-top: 15px;
			}
		}

		&__copy-and-share-container {
			background-color: #FFFFFF;
            margin-top: 40px;
			padding: 40px 136px 40px 136px;
			border-radius: 24px;

			&__title {
				font-size: 26px;
				line-height: 16px;
				color: #354049;
				text-align: center;
				font-family: 'font_regular';
				font-weight: 900;
				padding-bottom: 22px;
			}

			&__link-holder {
				display: flex;
				justify-content: space-between;
				align-items: center;
				padding: 16px 15px 16px 21px;
				background-color: #F5F6FA;
				border-radius: 6px;
				margin-bottom: 24px;

				&__link {
					font-size: 16px;
					line-height: 134%;
					color: #494949;
				}

				.copy-button{
					width: 140px;
					height: 46px;
					display: flex;
					align-items: center;
					justify-content: center;
					background-color: #2683FF;
					color: #FFFFFF;
					border-radius: 6px;
					font-weight: 900;
					font-size: 16px;
					line-height: 23px;

					&:hover {
						box-shadow: 0 4px 14px rgba(38, 131, 255, 0.3);
						cursor: pointer;
					}
				}
			}

			&__share-links {
				display: flex;
				flex-direction: row;
				justify-content: space-between;
				align-items: center;

				&__info{
					color: rgba(116, 116, 116, 0.5);
					font-size: 17px;
					line-height: 16px;
				}

				.twitter-link,
				.facebook-link,
				.linkedin-link,
				.email-link {
					display: flex;
					justify-content: space-between;
					align-items: center;
					padding: 10px 25px 10px 15px;
					border: 1px solid #AFB7C1;
					border-radius: 6px;
					height: 20px;
					cursor: pointer;

					p {
						font-size: 14px;
						color: #494949;
					}
				}

				.twitter-link,
				.email-link {
					width: 80px;
				}

				.facebook-link,
				.linkedin-link {
					width: 92px;
				}
			}
		}

		&__help {
			text-align: center;
			max-width: 778px;
			font-size: 13px;
			line-height: 24px;
			color: #959595;
			margin: 40px 0 0 81px;
			justify-content: center;

			a {
				color: #0000EE;
                cursor: pointer;

                &:hover {
                    text-decoration: underline;
                }
			}
		}
	}

    .icon-container {
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;
    }

    .underline {
        margin-top: 70px;
        margin-bottom: 40px;
        min-height: 1px;
        background-color: #C5C5C5;
    }

	@media screen and (max-height: 1200px) {
		.referral-container {
			overflow-y: scroll;
			height: 950px;
		}
	}

	@media screen and (max-height: 1180px) {
		.referral-container {
			overflow-y: scroll;
			height: 930px;
		}
	}

	@media screen and (max-height: 1160px) {
		.referral-container {
			overflow-y: scroll;
			height: 910px;
		}
	}

	@media screen and (max-height: 1140px) {
		.referral-container {
			overflow-y: scroll;
			height: 890px;
		}
	}

	@media screen and (max-height: 1120px) {
		.referral-container {
			overflow-y: scroll;
			height: 870px;
		}
	}

	@media screen and (max-height: 1100px) {
		.referral-container {
			overflow-y: scroll;
			height: 850px;
		}
	}

	@media screen and (max-height: 1080px) {
		.referral-container {
			overflow-y: scroll;
			height: 830px;
		}
	}

	@media screen and (max-height: 1060px) {
		.referral-container {
			overflow-y: scroll;
			height: 810px;
		}
	}

	@media screen and (max-height: 1040px) {
		.referral-container {
			overflow-y: scroll;
			height: 790px;
		}
	}

	@media screen and (max-height: 1020px) {
		.referral-container {
			overflow-y: scroll;
			height: 770px;
		}
	}

	@media screen and (max-height: 1000px) {
		.referral-container {
			overflow-y: scroll;
			height: 750px;
		}
	}

	@media screen and (max-height: 980px) {
		.referral-container {
			overflow-y: scroll;
			height: 730px;
		}
	}

	@media screen and (max-height: 960px) {
		.referral-container {
			overflow-y: scroll;
			height: 710px;
		}
	}

	@media screen and (max-height: 940px) {
		.referral-container {
			overflow-y: scroll;
			height: 690px;
		}
	}

	@media screen and (max-height: 920px) {
		.referral-container {
			overflow-y: scroll;
			height: 670px;
		}
	}

	@media screen and (max-height: 900px) {
		.referral-container {
			overflow-y: scroll;
			height: 650px;
		}
	}

	@media screen and (max-height: 880px) {
		.referral-container {
			overflow-y: scroll;
			height: 630px;
		}
	}

	@media screen and (max-height: 860px) {
		.referral-container {
			overflow-y: scroll;
			height: 610px;
		}
	}

	@media screen and (max-height: 840px) {
		.referral-container {
			overflow-y: scroll;
			height: 590px;
		}
	}

	@media screen and (max-height: 820px) {
		.referral-container {
			overflow-y: scroll;
			height: 570px;
		}
	}

	@media screen and (max-height: 800px) {
		.referral-container {
			overflow-y: scroll;
			height: 550px;
		}
	}
</style>
