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
import OwnCloudIcon from '@/assets/apps/owncloud.png';
import LivepeerIcon from '@/assets/apps/livepeer.png';
import BunnyCDNIcon from '@/assets/apps/bunnycdn.png';
import CometIcon from '@/assets/apps/comet.png';
import FastlyIcon from '@/assets/apps/fastly.jpg';
import GlobusIcon from '@/assets/apps/globus.png';
import HuggingFaceIcon from '@/assets/apps/huggingface.svg';
import KerberosIcon from '@/assets/apps/kerberos.png';
import RcloneIcon from '@/assets/apps/rclone.png';
import UpdraftPlusIcon from '@/assets/apps/updraftplus.png';
import DuplicatiIcon from '@/assets/apps/duplicati.png';
import ResticIcon from '@/assets/apps/restic.png';
import StarfishIcon from '@/assets/apps/starfish.png';

export enum AppCategory {
    All = 'All',
    FileManagement = 'File Management',
    BackupRecovery = 'Backup & Recovery',
    ContentDelivery = 'Content Delivery',
    Scientific = 'Scientific',
    AI = 'AI',
}

export type Application = {
    name: string
    description: string
    category: AppCategory
    src: string
    docs: string
}

export const applications: Application[] = [
    {
        name: 'TrueNAS - iX Systems',
        description: 'TrueNAS is a network attached storage (NAS) solution that allows for an off-site backup to your Storj account.',
        category: AppCategory.BackupRecovery,
        src: TrueNASIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/ix-systems-truenas',
    },
    {
        name: 'Hammerspace',
        description: 'Create a global data environment between Storj and all connected data as a single, easily accessible dataset.',
        category: AppCategory.FileManagement,
        src: HammerspaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hammerspace',
    },
    {
        name: 'OwnCloud',
        description: 'ownCloud Infinite Scale is a real-time content collaboration allowing you to use Storj as the primary storage location.',
        category: AppCategory.FileManagement,
        src: OwnCloudIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/ocis',
    },
    {
        name: 'Livepeer',
        description: 'Add live and on-demand video experiences by transcoding videos with Livepeer, and storing your media with Storj.',
        category: AppCategory.ContentDelivery,
        src: LivepeerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/livepeer',
    },
    {
        name: 'Acronis',
        description: 'Reliable backup and disaster recovery solutions for data archiving and organization, seamlessly integrating with Storj.',
        category: AppCategory.BackupRecovery,
        src: AcronisIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/acronis',
    },
    {
        name: 'Iconik',
        description: 'A cloud-native solution for media collaboration that integrates with Storj, to allow secure storage of your media assets.',
        category: AppCategory.FileManagement,
        src: IconikIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/iconik',
    },
    {
        name: 'Photos+',
        description: 'A beautfully designed app for iOS, Android, and Mac to store and manage your photos and videos in your Storj account.',
        category: AppCategory.FileManagement,
        src: PhotosPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/photos-plus',
    },
    {
        name: 'Veeam',
        description: 'All-in-one backup, recovery, and data security solution integrating Storj for secure, scalable backup and archiving.',
        category: AppCategory.BackupRecovery,
        src: VeeamIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/veeam',
    },
    {
        name: 'Splunk',
        description: 'Collect, index, and search all types of data generated by your business, and archive it to Storj.',
        category: AppCategory.BackupRecovery,
        src: SplunkIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/splunk',
    },
    {
        name: 'Arq Backup',
        description: 'Arq backs up your filesystem with perfect, point-in-time back ups of your files to Storj.',
        category: AppCategory.BackupRecovery,
        src: ArqIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/arq',
    },
    {
        name: 'Rclone',
        description: 'Open source command-line interface for sync, backup, restore, mirror, mount, and analyzing your Storj cloud storage.',
        category: AppCategory.BackupRecovery,
        src: RcloneIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/rclone',
    },
    {
        name: 'Zerto',
        description: 'Simple, scalable disaster recovery and data protection, integrating with Storj for cloud data management.',
        category: AppCategory.BackupRecovery,
        src: ZertoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/zerto',
    },
    {
        name: 'Unitrends',
        description: 'Backup and recovery platform allowing you to easily manage backups between Storj, SaaS, data centers and endpoints.',
        category: AppCategory.BackupRecovery,
        src: UnitrendsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/unitrends',
    },
    {
        name: 'S3FS',
        description: 's3fs allows Linux, macOS, and FreeBSD to mount your Storj buckets and work like on a local file system.',
        category: AppCategory.FileManagement,
        src: S3FSIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/s3fs',
    },
    {
        name: 'Rucio',
        description: 'Organize, manage, and access your Storj data at scale with Rucio, an open-source framework for scientific collaboration.',
        category: AppCategory.Scientific,
        src: RucioIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/rucio',
    },
    {
        name: 'MSP360',
        description: 'Cross-platform storage, backup, and disaster recovery solution integrating Storj cloud storage for managed service providers.',
        category: AppCategory.BackupRecovery,
        src: MSP360Icon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/msp360',
    },
    {
        name: 'LucidLink',
        description: 'LucidLink integrates with Storj for fast and secure file-streaming, enabling real-time collaboration on creative projects.',
        category: AppCategory.FileManagement,
        src: LucidLinkIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/lucidlink',
    },
    {
        name: 'Elements',
        description: 'Store, manage and collaborate on your Storj media content using a secure and scalable asset management platform.',
        category: AppCategory.FileManagement,
        src: ElementsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/elements',
    },
    {
        name: 'Docker',
        description: 'Storj supports custom Content-Type for any key, so it can be used as a container registry to distribute container images.',
        category: AppCategory.ContentDelivery,
        src: DockerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/docker',
    },
    {
        name: 'Pixelfed',
        description: 'Learn how to set up Pixelfed decentralized social media platform to Storj.',
        category: AppCategory.ContentDelivery,
        src: PixelfedIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/pixelfed',
    },
    {
        name: 'Mastodon',
        description: 'Learn how to set up Mastodon decentralized social media platform to Storj.',
        category: AppCategory.ContentDelivery,
        src: MastodonIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/mastodon',
    },
    {
        name: 'Dataverse',
        description: 'Dataverse integrates with Storj to provide researchers secure archiving, controlling, and sharing large research data sets.',
        category: AppCategory.Scientific,
        src: DataverseIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/dataverse',
    },
    {
        name: 'CyberDuck',
        description: 'Open-source cloud storage browser app for macOS, Windows, and Linux, that supports Storj.',
        category: AppCategory.FileManagement,
        src: CyberDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        name: 'Atempo (Miria)',
        description: 'Atempo provides high-performance backup, replication, synchronization, and archiving of large data sets to Storj.',
        category: AppCategory.BackupRecovery,
        src: AtempoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/atempo-miria',
    },
    {
        name: 'Mountain Duck',
        description: 'Mount Storj as a virtual drive on your computer. Open your Storj files with any application, and work like on a local disk.',
        category: AppCategory.FileManagement,
        src: MountainDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/mountainduck',
    },
    {
        name: 'Bunny CDN',
        description: 'Set up a static website hosting with Storj, using Bunny CDN as a content delivery network providing a caching layer.',
        category: AppCategory.ContentDelivery,
        src: BunnyCDNIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/bunny',
    },
    {
        name: 'Fastly',
        description: 'Distribute your content among the Fastly edge cloud service, using your Storj buckets as a source of content.',
        category: AppCategory.ContentDelivery,
        src: FastlyIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/fastly',
    },
    {
        name: 'Comet',
        description: 'Flexible backup platform, integrating with Storj, as a backup and storage destination, to protect and restore data.',
        category: AppCategory.BackupRecovery,
        src: CometIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/comet-backup',
    },
    {
        name: 'Globus',
        description: 'Open-source platform for collaboration and management, for transferring, sharing, and discovering your data on Storj.',
        category: AppCategory.Scientific,
        src: GlobusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/globus',
    },
    {
        name: 'HuggingFace',
        description: 'Train and deploy open-source AI models, while saving and loading datasets to and from Storj.',
        category: AppCategory.AI,
        src: HuggingFaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hugging-face',
    },
    {
        name: 'FileZilla',
        description: 'Learn how to set up FileZilla to transfer your files over Storj, or integrate with FileZilla Pro.',
        category: AppCategory.FileManagement,
        src: FileZillaIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/filezilla',
    },
    {
        name: 'Kerberos.io',
        description: 'Open-source platform for video analytics and management. Integrate with Storj to store your Kerberos Vault video files.',
        category: AppCategory.FileManagement,
        src: KerberosIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/kerberos-vault',
    },
    {
        name: 'UpdraftPlus',
        description: 'Automatically backup your Wordpress site, and use Storj as a remote storage destination.',
        category: AppCategory.BackupRecovery,
        src: UpdraftPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/wordpress-site-with-updraftplus',
    },
    {
        name: 'Duplicati',
        description: 'Free and open source backup client, to store encrypted, incremental, compressed backups to Storj.',
        category: AppCategory.BackupRecovery,
        src: DuplicatiIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/duplicati',
    },
    {
        name: 'Restic',
        description: 'Restic is an open source command-line backup tool, optimized for securely backing up data to Storj.',
        category: AppCategory.BackupRecovery,
        src: ResticIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/restic',
    },
    {
        name: 'Starfish',
        description: 'Unstructured data management and metadata for files and objects. Integrate with Storj for large-scale file management.',
        category: AppCategory.FileManagement,
        src: StarfishIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/starfish',
    },
];
