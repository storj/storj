// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import VeeamIcon from '@/assets/apps/veeam.png';
import TrueNASIcon from '@/assets/apps/truenas.png';
import SplunkIcon from '@/assets/apps/splunk.png';
import ArqIcon from '@/assets/apps/arq.png';
import ZertoIcon from '@/assets/apps/zerto.png';
import UnitrendsIcon from '@/assets/apps/unitrends.png';
import S3FSIcon from '@/assets/apps/s3fs.png';
import RucioIcon from '@/assets/apps/rucio.png';
import PhotosPlusIcon from '@/assets/apps/photos+.png';
import MSP360Icon from '@/assets/apps/msp360.png';
import LucidLinkIcon from '@/assets/apps/lucidlink.png';
import IconikIcon from '@/assets/apps/iconik.png';
import HammerspaceIcon from '@/assets/apps/hammerspace.png';
import ElementsIcon from '@/assets/apps/elements.png';
import DockerIcon from '@/assets/apps/docker.png';
import PixelfedIcon from '@/assets/apps/pixelfed.png';
import MastodonIcon from '@/assets/apps/mastodon.png';
import DataverseIcon from '@/assets/apps/dataverse.png';
import CyberDuckIcon from '@/assets/apps/cyberduck.png';
import AtempoIcon from '@/assets/apps/atempo.png';
import AcronisIcon from '@/assets/apps/acronis.png';
import MountainDuckIcon from '@/assets/apps/mountainduck.png';
import FileZillaIcon from '@/assets/apps/filezilla.svg';
import GBLabsIcon from '@/assets/apps/gblabs.png';
import AdSignalIcon from '@/assets/apps/adsignal.png';
import OwnCloudIcon from '@/assets/apps/owncloud.png';
import LivepeerIcon from '@/assets/apps/livepeer.png';
import BunnyCDNIcon from '@/assets/apps/bunnycdn.png';
import CometIcon from '@/assets/apps/comet.png';
import FastlyIcon from '@/assets/apps/fastly.jpg';
import GlobusIcon from '@/assets/apps/globus.png';
import HuggingFaceIcon from '@/assets/apps/huggingface.svg';
import KerberosIcon from '@/assets/apps/kerberos.png';

export enum AppCategory {
    All = 'All',
    FileManagement = 'File Management',
    BackupRecovery = 'Backup & Recovery',
    ContentDistribution = 'Content Distribution',
    CDN = 'CDN',
    AI = 'AI',
    Logs = 'Logs',
    SocialMedia = 'Social Media',
}

export type Application = {
    title: string
    description: string
    category: AppCategory
    src: string
    docs: string
}

export const applications: Application[] = [
    {
        title: 'TrueNAS - iX Systems',
        description: 'Back up a single instance or keep several TrueNAS deployments synchronized using a single source of truth.',
        category: AppCategory.BackupRecovery,
        src: TrueNASIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/ix-systems-truenas',
    },
    {
        title: 'Nebula by GB Labs',
        description: 'Cloud-based media storage solution powered by Storj. Work directly from the GB Labs Cloud with project files that are always in sync.',
        category: AppCategory.FileManagement,
        src: GBLabsIcon,
        docs: 'https://www.storj.io/partner-solutions/gb-labs',
    },
    {
        title: 'Hammerspace',
        description: 'Hammerspace and Storj bring more flexibility and agility to your global data landscape.',
        category: AppCategory.FileManagement,
        src: HammerspaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hammerspace',
    },
    {
        title: 'Ad Signal Match',
        description: 'Deduplicate assets and save on AI costs with Ad Signal Match. Transfer only your unique content to Storj.',
        category: AppCategory.FileManagement,
        src: AdSignalIcon,
        docs: 'https://www.storj.io/partner-solutions/ad-signal',
    },
    {
        title: 'OwnCloud',
        description: 'oCIS, or ownCloud Infinite Scale, is a cutting-edge technology platform for building cloud-native file sync and share applications.',
        category: AppCategory.FileManagement,
        src: OwnCloudIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/ocis',
    },
    {
        title: 'Livepeer',
        description: 'Add live and on-demand video experiences by transcoding videos with Livepeer and storing your media with Storj.',
        category: AppCategory.BackupRecovery,
        src: LivepeerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/livepeer',
    },
    {
        title: 'Acronis',
        description: 'Reliable backup and disaster recovery solutions for data archiving and organization, seamlessly integrating with Storj.',
        category: AppCategory.BackupRecovery,
        src: AcronisIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/acronis',
    },
    {
        title: 'Iconik',
        description: 'Iconik is easy to use and intuitive with a clean and intuitive platform that organizes your media and makes it searchable.',
        category: AppCategory.FileManagement,
        src: IconikIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/iconik',
    },
    {
        title: 'Photos+',
        description: 'Store and manage your photos and videos in your own cloud storage account.',
        category: AppCategory.FileManagement,
        src: PhotosPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/photos-plus',
    },
    {
        title: 'Veeam',
        description: 'All-in-one backup, recovery, and data security solution that serves both on-premises and cloud storage.',
        category: AppCategory.BackupRecovery,
        src: VeeamIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/veeam',
    },
    {
        title: 'Splunk',
        description: 'Splunk is a data analytics platform that provides data-driven insights across all aspects of a company.',
        category: AppCategory.Logs,
        src: SplunkIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/splunk',
    },
    {
        title: 'Arq Backup',
        description: 'Arq backs up your filesystem with perfect, point-in-time backups of your files.',
        category: AppCategory.BackupRecovery,
        src: ArqIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/arq',
    },
    {
        title: 'Zerto',
        description: 'Easily incorporate immutability into a complete disaster recovery strategy.',
        category: AppCategory.BackupRecovery,
        src: ZertoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/zerto',
    },
    {
        title: 'Unitrends',
        description: 'Automate manual tasks, eliminate management complexity, and deliver tested hardware and software resilience.',
        category: AppCategory.BackupRecovery,
        src: UnitrendsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/unitrends',
    },
    {
        title: 'S3FS',
        description: 's3fs allows Linux, macOS, and FreeBSD to mount an S3 bucket via FUSE.',
        category: AppCategory.FileManagement,
        src: S3FSIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/s3fs',
    },
    {
        title: 'Rucio',
        description: 'Organize, manage, and access your data at scale with Rucio, an open-source framework for scientific collaboration.',
        category: AppCategory.FileManagement,
        src: RucioIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/rucio',
    },
    {
        title: 'MSP360',
        description: 'MSP360 is a best-in-class IT management platform for MSPs and internal IT teams.',
        category: AppCategory.BackupRecovery,
        src: MSP360Icon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/msp360',
    },
    {
        title: 'LucidLink',
        description: 'Using Storj with LucidLink provides resilient cloud object storage with blazing performance and zero-trust security.',
        category: AppCategory.FileManagement,
        src: LucidLinkIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/lucidlink',
    },
    {
        title: 'Elements',
        description: 'Store, manage and collaborate on media content using a secure and scalable asset management platform.',
        category: AppCategory.FileManagement,
        src: ElementsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/elements',
    },
    {
        title: 'Docker',
        description: 'Storj supports custom Content-Type for any key, so it can be used as a container registry to distribute container images.',
        category: AppCategory.ContentDistribution,
        src: DockerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/docker',
    },
    {
        title: 'Pixelfed',
        description: 'Learn how to set up Pixelfed decentralized social media platform to Storj.',
        category: AppCategory.SocialMedia,
        src: PixelfedIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/pixelfed',
    },
    {
        title: 'Mastodon',
        description: 'Here is how you can set up Mastodon decentralized social media platform to Storj.',
        category: AppCategory.SocialMedia,
        src: MastodonIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/mastodon',
    },
    {
        title: 'Dataverse',
        description: 'S3 credentials allow Dataverse to upload and download files from Storj as if it was using the S3 API.',
        category: AppCategory.FileManagement,
        src: DataverseIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/dataverse',
    },
    {
        title: 'CyberDuck',
        description: 'Open-source cloud storage browser app for macOS, Windows, and Linux that supports Storj.',
        category: AppCategory.FileManagement,
        src: CyberDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        title: 'Atempo',
        description: 'Integrate with Storj to reduce storage costs without sacrificing security or performance for media archival with global access.',
        category: AppCategory.BackupRecovery,
        src: AtempoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/atempo-miria',
    },
    {
        title: 'Mountain Duck',
        description: 'Add Storj as a disk on your computer. Open your Storj files with any application and work like on a local disk.',
        category: AppCategory.FileManagement,
        src: MountainDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        title: 'Bunny CDN',
        description: 'Set up a static website hosting with Storj using Bunny CDN as a content delivery network providing a caching layer.',
        category: AppCategory.CDN,
        src: BunnyCDNIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/bunny',
    },
    {
        title: 'Fastly',
        description: 'Distribute your content among the Fastly edge cloud service using your Storj buckets as a source of content.',
        category: AppCategory.CDN,
        src: FastlyIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/fastly',
    },
    {
        title: 'Comet',
        description: 'Flexible backup platform integrates with Storj as a backup and storage destination to protect and restore data.',
        category: AppCategory.BackupRecovery,
        src: CometIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/comet',
    },
    {
        title: 'Globus',
        description: 'Seamless collaboration and management for data transfer, sharing, and discovery, all into one unified open-source platform.',
        category: AppCategory.FileManagement,
        src: GlobusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/globus',
    },
    {
        title: 'HuggingFace',
        description: 'Train and deploy open-source AI models, while saving and loading datasets to and from Storj.',
        category: AppCategory.AI,
        src: HuggingFaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hugging-face',
    },
    {
        title: 'FileZilla',
        description: 'Learn how to set up FileZilla to transfer files over Storj DCS or integrate FileZilla Pro.',
        category: AppCategory.FileManagement,
        src: FileZillaIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/filezilla',
    },
    {
        title: 'Kerberos.io',
        description: 'Open-source platform for video analytics and management. Integrate with Storj to store your Kerberos Vault video files.',
        category: AppCategory.FileManagement,
        src: KerberosIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/kerberos-vault',
    },
];
