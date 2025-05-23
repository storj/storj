// Copyright (C) 2025 Storj Labs, Inc.
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
import CommvaultIcon from '@/assets/apps/commvault.png';
import StorjIcon from '@/assets/apps/storj.svg';
import AdobeIcon from '@/assets/apps/adobepremiere.png';
import AdSignalIcon from '@/assets/apps/adsignal.png';
import AmoveIcon from '@/assets/apps/amove.png';
import BeamIcon from '@/assets/apps/beam.png';
import CuttingRoomIcon from '@/assets/apps/cuttingroom.jpeg';
import GBLabsIcon from '@/assets/apps/gblabs.png';
import HedgeIcon from '@/assets/apps/hedge.webp';
import ImagineProductsIcon from '@/assets/apps/imagine.png';
import OpenDrivesIcon from '@/assets/apps/opendrives.webp';
import MasvIcon from '@/assets/apps/masv.jpeg';
import OrtanaIcon from '@/assets/apps/ortana.png';
import SigniantIcon from '@/assets/apps/signiant.jpeg';
import VarnishIcon from '@/assets/apps/varnish.webp';

export enum AppCategory {
    Featured = 'Featured',
    All = 'All',
    Media = 'Media',
    FileManagement = 'File Management',
    BackupRecovery = 'Backup & Recovery',
    ContentDelivery = 'Content Delivery',
    Scientific = 'Scientific',
    AI = 'AI',
}

export type Application = {
    name: string
    description: string
    categories: AppCategory[]
    src: string
    docs: string
};

export const UplinkApp: Application = {
    name: 'Storj Uplink CLI',
    description: 'Official Storj command-line application that allows you to access, upload, download, and manage your data.',
    categories: [AppCategory.Featured],
    src: StorjIcon,
    docs: 'https://docs.storj.io/dcs/api/uplink-cli/installation',
};

export const ObjectMountApp: Application = {
    name: 'Storj Object Mount',
    description: 'Access files stored on Storj as if they were on local disk, allowing real-time editing without downloading entire files.',
    categories: [AppCategory.Media, AppCategory.FileManagement],
    src: StorjIcon,
    docs: 'https://storj.dev/object-mount',
};

export const applications: Application[] = [
    ObjectMountApp,
    UplinkApp,
    {
        name: 'TrueNAS - iX Systems',
        description: 'TrueNAS is a network attached storage (NAS) solution that allows for an off-site backup to your Storj account.',
        categories: [AppCategory.BackupRecovery, AppCategory.Media],
        src: TrueNASIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/truenas#connecting-true-nas-to-storj',
    },
    {
        name: 'Hammerspace',
        description: 'Create a global data environment between Storj and all connected data as a single, easily accessible dataset.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: HammerspaceIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/hammerspace#integrating-hammerspace-with-storj',
    },
    {
        name: 'OwnCloud',
        description: 'ownCloud Infinite Scale is a real-time content collaboration allowing you to use Storj as the primary storage location.',
        categories: [AppCategory.FileManagement],
        src: OwnCloudIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/ocis#connecting-to-storj-via-o-cis-s3-ng',
    },
    {
        name: 'Livepeer',
        description: 'Add live and on-demand video experiences by transcoding videos with Livepeer, and storing your media with Storj.',
        categories: [AppCategory.ContentDelivery, AppCategory.Media],
        src: LivepeerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/livepeer',
    },
    {
        name: 'Acronis',
        description: 'Reliable backup and disaster recovery solutions for data archiving and organization, seamlessly integrating with Storj.',
        categories: [AppCategory.BackupRecovery],
        src: AcronisIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/acronis',
    },
    {
        name: 'Iconik',
        description: 'A cloud-native solution for media collaboration that integrates with Storj, to allow secure storage of your media assets.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: IconikIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/iconik#integrating-iconik-with-storj',
    },
    {
        name: 'Photos+',
        description: 'A beautfully designed app for iOS, Android, and Mac to store and manage your photos and videos in your Storj account.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: PhotosPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/photos-plus',
    },
    {
        name: 'Veeam',
        description: 'All-in-one backup, recovery, and data security solution integrating Storj for secure, scalable backup and archiving.',
        categories: [AppCategory.BackupRecovery],
        src: VeeamIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/veeam',
    },
    {
        name: 'Splunk',
        description: 'Collect, index, and search all types of data generated by your business, and archive it to Storj.',
        categories: [AppCategory.BackupRecovery],
        src: SplunkIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/splunk#connecting-splunk-to-storj',
    },
    {
        name: 'Arq Backup',
        description: 'Arq backs up your filesystem with perfect, point-in-time back ups of your files to Storj.',
        categories: [AppCategory.BackupRecovery],
        src: ArqIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/arq',
    },
    {
        name: 'Rclone',
        description: 'Open source command-line interface for sync, backup, restore, mirror, mount, and analyzing your Storj cloud storage.',
        categories: [AppCategory.BackupRecovery],
        src: RcloneIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/rclone',
    },
    {
        name: 'Zerto',
        description: 'Simple, scalable disaster recovery and data protection, integrating with Storj for cloud data management.',
        categories: [AppCategory.BackupRecovery],
        src: ZertoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/zerto',
    },
    {
        name: 'Unitrends',
        description: 'Backup and recovery platform allowing you to easily manage backups between Storj, SaaS, data centers and endpoints.',
        categories: [AppCategory.BackupRecovery],
        src: UnitrendsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/unitrends',
    },
    {
        name: 'S3FS',
        description: 's3fs allows Linux, macOS, and FreeBSD to mount your Storj buckets and work like on a local file system.',
        categories: [AppCategory.FileManagement],
        src: S3FSIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/s3fs',
    },
    {
        name: 'Rucio',
        description: 'Organize, manage, and access your Storj data at scale with Rucio, an open-source framework for scientific collaboration.',
        categories: [AppCategory.Scientific],
        src: RucioIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/rucio',
    },
    {
        name: 'MSP360',
        description: 'Cross-platform storage, backup, and disaster recovery solution integrating Storj cloud storage for managed service providers.',
        categories: [AppCategory.BackupRecovery],
        src: MSP360Icon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/msp360',
    },
    {
        name: 'LucidLink',
        description: 'LucidLink integrates with Storj for fast and secure file-streaming, enabling real-time collaboration on creative projects.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: LucidLinkIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/lucidlink',
    },
    {
        name: 'Elements',
        description: 'Store, manage and collaborate on your Storj media content using a secure and scalable asset management platform.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: ElementsIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/elements',
    },
    {
        name: 'Docker',
        description: 'Storj supports custom Content-Type for any key, so it can be used as a container registry to distribute container images.',
        categories: [AppCategory.ContentDelivery],
        src: DockerIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/docker',
    },
    {
        name: 'Pixelfed',
        description: 'Learn how to set up Pixelfed decentralized social media platform to Storj.',
        categories: [AppCategory.ContentDelivery],
        src: PixelfedIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/pixelfed',
    },
    {
        name: 'Mastodon',
        description: 'Learn how to set up Mastodon decentralized social media platform to Storj.',
        categories: [AppCategory.ContentDelivery],
        src: MastodonIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/mastodon',
    },
    {
        name: 'Dataverse',
        description: 'Dataverse integrates with Storj to provide researchers secure archiving, controlling, and sharing large research data sets.',
        categories: [AppCategory.Scientific],
        src: DataverseIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/dataverse',
    },
    {
        name: 'CyberDuck',
        description: 'Open-source cloud storage browser app for macOS, Windows, and Linux, that supports Storj.',
        categories: [AppCategory.FileManagement],
        src: CyberDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/cyberduck',
    },
    {
        name: 'Atempo (Miria)',
        description: 'Atempo provides high-performance backup, replication, synchronization, and archiving of large data sets to Storj.',
        categories: [AppCategory.BackupRecovery, AppCategory.Media],
        src: AtempoIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/atempo-miria',
    },
    {
        name: 'Mountain Duck',
        description: 'Mount Storj as a virtual drive on your computer. Open your Storj files with any application, and work like on a local disk.',
        categories: [AppCategory.FileManagement],
        src: MountainDuckIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/mountainduck',
    },
    {
        name: 'Bunny CDN',
        description: 'Set up a static website hosting with Storj, using Bunny CDN as a content delivery network providing a caching layer.',
        categories: [AppCategory.ContentDelivery],
        src: BunnyCDNIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/bunny',
    },
    {
        name: 'Fastly',
        description: 'Distribute your content among the Fastly edge cloud service, using your Storj buckets as a source of content.',
        categories: [AppCategory.ContentDelivery],
        src: FastlyIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/fastly',
    },
    {
        name: 'Comet',
        description: 'Flexible backup platform, integrating with Storj, as a backup and storage destination, to protect and restore data.',
        categories: [AppCategory.BackupRecovery],
        src: CometIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/comet-backup',
    },
    {
        name: 'Globus',
        description: 'Open-source platform for collaboration and management, for transferring, sharing, and discovering your data on Storj.',
        categories: [AppCategory.Scientific],
        src: GlobusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/globus',
    },
    {
        name: 'HuggingFace',
        description: 'Train and deploy open-source AI models, while saving and loading datasets to and from Storj.',
        categories: [AppCategory.AI],
        src: HuggingFaceIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/hugging-face',
    },
    {
        name: 'FileZilla',
        description: 'Learn how to set up FileZilla to transfer your files over Storj, or integrate with FileZilla Pro.',
        categories: [AppCategory.FileManagement],
        src: FileZillaIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/filezilla',
    },
    {
        name: 'Kerberos.io',
        description: 'Open-source platform for video analytics and management. Integrate with Storj to store your Kerberos Vault video files.',
        categories: [AppCategory.FileManagement, AppCategory.Media],
        src: KerberosIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/kerberos-vault',
    },
    {
        name: 'UpdraftPlus',
        description: 'Automatically backup your Wordpress site, and use Storj as a remote storage destination.',
        categories: [AppCategory.BackupRecovery],
        src: UpdraftPlusIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/wordpress-site-with-updraftplus',
    },
    {
        name: 'Duplicati',
        description: 'Free and open source backup client, to store encrypted, incremental, compressed backups to Storj.',
        categories: [AppCategory.BackupRecovery],
        src: DuplicatiIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/duplicati',
    },
    {
        name: 'Restic',
        description: 'Restic is an open source command-line backup tool, optimized for securely backing up data to Storj.',
        categories: [AppCategory.BackupRecovery],
        src: ResticIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/restic',
    },
    {
        name: 'Starfish',
        description: 'Unstructured data management and metadata for files and objects. Integrate with Storj for large-scale file management.',
        categories: [AppCategory.FileManagement],
        src: StarfishIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/starfish',
    },
    {
        name: 'Commvault',
        description: 'Reduce downtime, maintain operations and drive resiliency with Commvault and Storj.',
        categories: [AppCategory.BackupRecovery],
        src: CommvaultIcon,
        docs: 'https://docs.storj.io/dcs/third-party-tools/commvault',
    },
    {
        name: 'Adobe Premiere',
        description: 'Professional video editing software that connects to Storj for direct project file access and collaborative editing.',
        categories: [AppCategory.Media],
        src: AdobeIcon,
        docs: '',
    },
    {
        name: 'AdSignal',
        description: 'Eliminate duplicate content and reduce AI costs by using precise content tagging to ensure efficient storage with Storj.',
        categories: [AppCategory.Media],
        src: AdSignalIcon,
        docs: '',
    },
    {
        name: 'Ortana',
        description: 'Enterprise media asset management platform that orchestrates media workflows while using Storj for cost-effective cloud storage.',
        categories: [AppCategory.Media],
        src: OrtanaIcon,
        docs: '',
    },
    {
        name: 'GB Labs',
        description: 'Intelligent storage solutions with hybrid cloud capabilities, integrating with Storj for secure and accessible off-site backup.',
        categories: [AppCategory.Media, AppCategory.BackupRecovery],
        src: GBLabsIcon,
        docs: '',
    },
    {
        name: 'Varnish',
        description: 'High-performance HTTP accelerator that pairs with Storj to optimize content delivery networks for faster media distribution.',
        categories: [AppCategory.Media, AppCategory.ContentDelivery],
        src: VarnishIcon,
        docs: '',
    },
    {
        name: 'Hedge',
        description: 'Backup and safety copy software for media professionals that secures footage to Storj with transfer verification and metadata.',
        categories: [AppCategory.Media, AppCategory.BackupRecovery],
        src: HedgeIcon,
        docs: '',
    },
    {
        name: 'MASV',
        description: 'Fast file transfer service designed for large media files with direct integration to Storj for secure long-term storage.',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: MasvIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/MASV',
    },
    {
        name: 'Signiant',
        description: 'Enterprise file transfer solution optimized for moving large media assets quickly and securely between Storj and any location.',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: SigniantIcon,
        docs: 'https://storj.dev/dcs/third-party-tools/signiant',
    },
    {
        name: 'Amove',
        description: 'Accelerated media transfer solution that works with Storj for efficient media workflow and cloud storage..',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: AmoveIcon,
        docs: '',
    },
    {
        name: 'Cutting Room',
        description: 'Cloud-based media workflow platform that links creative teams with transparent integration to Storj for secure content storage.',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: CuttingRoomIcon,
        docs: '',
    },
    {
        name: 'Imagine Products',
        description: 'Software tools for media management, transcoding, and backup that seamlessly archive to Storj for distributed protection.',
        categories: [AppCategory.Media, AppCategory.BackupRecovery],
        src: ImagineProductsIcon,
        docs: '',
    },
    {
        name: 'Open Drives',
        description: 'Enterprise media storage and workflow platform that connects production environments to Storj for collaborative content access.',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: OpenDrivesIcon,
        docs: '',
    },
    {
        name: 'Beam Transfer',
        description: 'Professional file transfer solution with end-to-end encryption that uses Storj for cost-effective cloud storage integration.',
        categories: [AppCategory.Media, AppCategory.FileManagement],
        src: BeamIcon,
        docs: '',
    },
];
