using System;
using System.Collections.Generic;
using System.IO.Abstractions.TestingHelpers;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateIdentityDirTests
    {
        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select an identity folder.")]
        public void NullIdentityDir()
        {
            new CustomActionRunner().ValidateIdentityDir(null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select an identity folder.")]
        public void EmptyIdentityDir()
        {
            new CustomActionRunner().ValidateIdentityDir("");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "Folder 'X:\\Some\\Nonexistent\\Folder' does not exist.")]
        public void NonexistentIdentityDir()
        {
            new CustomActionRunner().ValidateIdentityDir("X:\\Some\\Nonexistent\\Folder");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "File 'ca.cert' not found in the selected folder.")]
        public void MissingCACertFile()
        {
            var fs = new MockFileSystem(new Dictionary<string, MockFileData>
            {
                { @"X:\\Some\\Identity\\Folder\\identity.cert", new MockFileData("") },
                { @"X:\\Some\\Identity\\Folder\\identity.key", new MockFileData("") }
            });
            new CustomActionRunner(fs).ValidateIdentityDir("X:\\Some\\Identity\\Folder");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "File 'identity.cert' not found in the selected folder.")]
        public void MissingIdentityCertFile()
        {
            var fs = new MockFileSystem(new Dictionary<string, MockFileData>
            {
                { @"X:\\Some\\Identity\\Folder\\ca.cert", new MockFileData("") },
                { @"X:\\Some\\Identity\\Folder\\identity.key", new MockFileData("") }
            });
            new CustomActionRunner(fs).ValidateIdentityDir("X:\\Some\\Identity\\Folder");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "File 'identity.key' not found in the selected folder.")]
        public void MissingIdentityKeyFile()
        {
            var fs = new MockFileSystem(new Dictionary<string, MockFileData>
            {
                { @"X:\\Some\\Identity\\Folder\\ca.cert", new MockFileData("") },
                { @"X:\\Some\\Identity\\Folder\\identity.cert", new MockFileData("") }
            });
            new CustomActionRunner(fs).ValidateIdentityDir("X:\\Some\\Identity\\Folder");
        }

        [TestMethod]
        public void ValidIdentityDir()
        {
            var fs = new MockFileSystem(new Dictionary<string, MockFileData>
            {
                { @"X:\\Some\\Identity\\Folder\\ca.cert", new MockFileData("") },
                { @"X:\\Some\\Identity\\Folder\\identity.cert", new MockFileData("") },
                { @"X:\\Some\\Identity\\Folder\\identity.key", new MockFileData("") },
            });
            new CustomActionRunner(fs).ValidateIdentityDir("X:\\Some\\Identity\\Folder");
        }
    }
}
