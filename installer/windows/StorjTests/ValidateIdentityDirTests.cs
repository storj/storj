﻿using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateIdentityDirTests
    {
        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "You must select an identity folder.")]
        public void NullIdentityDir()
        {
            CustomActionRunner.ValidateIdentityDir(null);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "You must select an identity folder.")]
        public void EmptyIdentityDir()
        {
            CustomActionRunner.ValidateIdentityDir("");
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "Folder 'X:\\Some\\Nonexistent\\Folder' does not exist.")]
        public void NonexistentIdentityDir()
        {
            CustomActionRunner.ValidateIdentityDir("X:\\Some\\Nonexistent\\Folder");
        }

        // TODO: add tests that mock the file system
    }
}
