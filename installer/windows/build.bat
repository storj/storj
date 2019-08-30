candle.exe -arch x64 StorageNode.wxs CustomInstallDir.wxs Config.wxs

light.exe -o storagenode.msi -ext WixUIExtension StorageNode.wixobj CustomInstallDir.wixobj Config.wixobj
