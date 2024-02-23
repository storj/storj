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

export enum AppCategory {
    All = 'All',
    FileManagement = 'File Management',
    BackupRecovery = 'Backup & Recovery',
    ContentDistribution = 'Content Distribution',
    Log = 'Log',
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
        title: 'Veeam',
        description: 'All-in-one backup, recovery, and data security solution that serves both on-premises and cloud storage.',
        category: AppCategory.BackupRecovery,
        src: VeeamIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/veeam',
    },
    {
        title: 'TrueNAS - iX Systems',
        description: 'Back up a single instance or keep several TrueNAS deployments synchronized using a single source of truth.',
        category: AppCategory.BackupRecovery,
        src: TrueNASIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/ix-systems-truenas',
    },
    {
        title: 'Splunk',
        description: 'Splunk is a data analytics platform that provides data-driven insights across all aspects of a company.',
        category: AppCategory.Log,
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
        title: 'Photos+',
        description: 'Store and manage your photos and videos in your own cloud storage account.',
        category: AppCategory.FileManagement,
        src: PhotosPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/photos-plus',
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
        title: 'Iconik',
        description: 'Iconik is easy to use and intuitive with a clean and intuitive platform that organizes your media and makes it searchable.',
        category: AppCategory.FileManagement,
        src: IconikIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/iconik',
    },
    {
        title: 'Hammerspace',
        description: 'Hammerspace and Storj bring more flexibility and agility to your global data landscape.',
        category: AppCategory.FileManagement,
        src: HammerspaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hammerspace',
    },
    {
        title: 'Elements',
        description: 'ELEMENTS S3 Integration can leverage Storj decentralized storage technology, providing enhanced security and scalability for users.',
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
        description: 'a libre server and cloud storage browser for Mac and Windows with support for Storj.',
        category: AppCategory.FileManagement,
        src: CyberDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        title: 'Atempo',
        description: 'Atempo’s integration with Storj reduces storage costs without sacrificing security or performance for media archival with global access.',
        category: AppCategory.BackupRecovery,
        src: AtempoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/atempo-miria',
    },
    {
        title: 'Acronis',
        description: 'Acronis is a leading cyber protection solution provider that delivers innovative backup, disaster recovery, and secure file sync and share services.',
        category: AppCategory.BackupRecovery,
        src: AcronisIcon,
        docs: 'https://docs.storj.io/dcs/how-tos/acronis-integration-guide',
    },
    {
        title: 'Mountain Duck',
        description: 'Add Storj as a disk on your computer. Open your Storj files with any application and work like on a local disk.',
        category: AppCategory.FileManagement,
        src: MountainDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        title: 'FileZilla',
        description: 'Learn how to set up FileZilla to transfer files over Storj DCS or integrate FileZilla Pro.',
        category: AppCategory.FileManagement,
        src: FileZillaIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/filezilla',
    },
];
