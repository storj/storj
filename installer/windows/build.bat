@echo off

rem count # of args
set argC=0
for %%x in (%*) do Set /A argC+=1

if not "%argC%"=="3" (
    echo usage: %~nx0 ^<storagenode.exe input path^> ^<storagenode-updater.exe input path^> ^<msi output path^>
    exit /B 1
)

rem copy the storagenode binaries to the installer project
copy %1 installer\windows\storagenode.exe
copy %2 installer\windows\storagenode-updater.exe

rem install NuGet packages
nuget install installer\windows\StorjTests\packages.config -o installer\windows\packages

rem build the installer
msbuild installer\windows\windows.sln /t:Build /p:Configuration=Release

rem cleanup copied binaries
del installer\windows\storagenode.exe
del installer\windows\storagenode-updater.exe

rem copy the MSI to the release dir
copy installer\windows\bin\Release\storagenode.msi %3
