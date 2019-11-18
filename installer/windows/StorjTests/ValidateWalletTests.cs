using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateWalletTests
    {
        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The payout address cannot be empty.")]
        public void NullWallet()
        {
            new CustomActionRunner().ValidateWallet(null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The payout address cannot be empty.")]
        public void EmptyWallet()
        {
            new CustomActionRunner().ValidateWallet("");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The payout address must start with a '0x' prefix.")]
        public void PrefixMissing()
        {
            new CustomActionRunner().ValidateWallet("e857955cfCd98bAe1073d4e314c3F9526799357A");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The payout address must have 40 characters after the '0x' prefix.")]
        public void TooShortWallet()
        {
            new CustomActionRunner().ValidateWallet("0xe857955cfCd98bAe1073d4e314c3F9526799357");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The payout address must have 40 characters after the '0x' prefix.")]
        public void TooLongWallet()
        {
            new CustomActionRunner().ValidateWallet("0xe857955cfCd98bAe1073d4e314c3F9526799357A0");
        }

        [TestMethod]
        public void ValidWallet()
        {
            new CustomActionRunner().ValidateWallet("0xe857955cfCd98bAe1073d4e314c3F9526799357A");
        }
    }
}
