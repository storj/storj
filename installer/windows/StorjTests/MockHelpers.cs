using System;
using System.IO.Abstractions;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Moq;
using Storj;

namespace StorjTests
{
    public class MockHelpers
    {
        public static IFileSystem MockFileSystemTotalSize(long totalSize)
        {
            var dir = Mock.Of<IDirectoryInfo>();
            Mock.Get(dir).Setup(d => d.Root).Returns(dir);
            Mock.Get(dir).Setup(d => d.FullName).Returns("X:\\");

            var dirFactory = Mock.Of<IDirectoryInfoFactory>();
            Mock.Get(dirFactory).Setup(d => d.FromDirectoryName(It.IsAny<string>())).Returns(dir);

            var drive = Mock.Of<IDriveInfo>();
            Mock.Get(drive).Setup(d => d.Name).Returns("X:\\");
            Mock.Get(drive).Setup(d => d.TotalSize).Returns(totalSize);

            var driveFactory = Mock.Of<IDriveInfoFactory>();
            Mock.Get(driveFactory).Setup(d => d.FromDriveName(It.IsAny<string>())).Returns(drive);

            var fs = Mock.Of<IFileSystem>();
            Mock.Get(fs).Setup(f => f.DriveInfo).Returns(driveFactory);
            Mock.Get(fs).Setup(f => f.DirectoryInfo).Returns(dirFactory);

            return fs;
        }
    }
}
