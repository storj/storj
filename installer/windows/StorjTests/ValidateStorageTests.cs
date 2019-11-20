using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateStorageTests
    {
        private const string StorageDir = "X:\\storage";

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void NullStorage()
        {
            new CustomActionRunner().ValidateStorage(null, StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void EmptyStorage()
        {
            new CustomActionRunner().ValidateStorage("", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "'some random text' is not a valid number.")]
        public void InvalidNumber()
        {
            new CustomActionRunner().ValidateStorage("some random text", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The allocated disk space cannot be less than 0.5 TB.")]
        public void TooSmall()
        {
            new CustomActionRunner().ValidateStorage("0.41", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "10000000 TB is too large value for allocated storage.")]
        public void TooLarge()
        {
            new CustomActionRunner().ValidateStorage("10000000", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The storage directory cannot be empty")]
        public void NullStorageDir()
        {
            new CustomActionRunner().ValidateStorage("3.14", null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The storage directory cannot be empty")]
        public void EmptyStorageDir()
        {
            new CustomActionRunner().ValidateStorage("3.14", "");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The disk size (0.2 TB) on the selected drive X:\\ is less than the allocated disk space plus the 10% overhead (3.45 TB total).")]
        public void NotEnoughSpace()
        {
            var fs = MockHelpers.MockFileSystemTotalSize(200 * CustomActionRunner.GB);
            new CustomActionRunner(fs).ValidateStorage("3.14", StorageDir);
        }

        [TestMethod]
        public void ValidStorage()
        {
            var fs = MockHelpers.MockFileSystemTotalSize(4 * CustomActionRunner.TB);
            new CustomActionRunner(fs).ValidateStorage("3.14", StorageDir);
        }
    }
}
