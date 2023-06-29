// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import TableLockedIcon from '@/../static/images/browser/tableLocked.svg';
import ColorBucketIcon from '@/../static/images/objects/colorBucket.svg';
import ColorFolderIcon from '@/../static/images/objects/colorFolder.svg';
import FileIcon from '@/../static/images/objects/file.svg';
import AudioIcon from '@/../static/images/objects/audio.svg';
import VideoIcon from '@/../static/images/objects/video.svg';
import ChevronLeftIcon from '@/../static/images/objects/chevronLeft.svg';
import GraphIcon from '@/../static/images/objects/graph.svg';
import PdfIcon from '@/../static/images/objects/pdf.svg';
import PictureIcon from '@/../static/images/objects/picture.svg';
import TxtIcon from '@/../static/images/objects/txt.svg';
import ZipIcon from '@/../static/images/objects/zip.svg';
import ProjectIcon from '@/../static/images/navigation/project.svg';

/**
 * Represents functionality to find object's type and appropriate icon for this type.
 */
export class ObjectType {
    private static icons = new Map<string, string>([
        ['locked', TableLockedIcon],
        ['bucket', ColorBucketIcon],
        ['folder', ColorFolderIcon],
        ['file', FileIcon],
        ['audio', AudioIcon],
        ['video', VideoIcon],
        ['back', ChevronLeftIcon],
        ['spreadsheet', GraphIcon],
        ['pdf', PdfIcon],
        ['image', PictureIcon],
        ['text', TxtIcon],
        ['archive', ZipIcon],
        ['project', ProjectIcon],
        ['shared-project', ProjectIcon],
    ]);

    static findIcon(type: string): string {
        return this.icons.get(type.toLowerCase()) || '';
    }

    static findType(object: string): string {
        const image = /(\.jpg|\.jpeg|\.png|\.gif)$/i;
        const video = /(\.mp4|\.mkv|\.mov)$/i;
        const audio = /(\.mp3|\.aac|\.wav|\.m4a)$/i;
        const text = /(\.txt|\.docx|\.doc|\.pages)$/i;
        const pdf = /(\.pdf)$/i;
        const archive = /(\.zip|\.tar.gz|\.7z|\.rar)$/i;
        const spreadsheet = /(\.xls|\.numbers|\.csv|\.xlsx|\.tsv)$/i;

        if (image.exec(object)) {
            return 'image';
        } else if (video.exec(object)) {
            return 'video';
        } else if (audio.exec(object)) {
            return 'audio';
        } else if (text.exec(object)) {
            return 'text';
        } else if (pdf.exec(object)) {
            return 'pdf';
        } else if (archive.exec(object)) {
            return 'archive';
        } else if (spreadsheet.exec(object)) {
            return 'spreadsheet';
        }
        return 'file';
    }
}
