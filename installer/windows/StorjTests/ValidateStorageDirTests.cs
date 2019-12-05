using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateStorageDirTests
    {
        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select a storage folder.")]
        public void NullStorageDir()
        {
            new CustomActionRunner().ValidateStorageDir(null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select a storage folder.")]
        public void EmptyStorageDir()
        {
            new CustomActionRunner().ValidateStorageDir("");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The selected drive 'X:\\' has only 200 GB disk size. The minimum required is 550 GB.")]
        public void NotEnoughSpace()
        {
            var fs = MockHelpers.MockFileSystemTotalSize(200 * CustomActionRunner.GB);
            new CustomActionRunner(fs).ValidateStorageDir("X:\\Storage");
        }

        [TestMethod]
        public void ValidStorageDir()
        {
            var fs = MockHelpers.MockFileSystemTotalSize(2 * CustomActionRunner.TB);
            new CustomActionRunner(fs).ValidateStorageDir("X:\\Storage");
        }
    }
}
