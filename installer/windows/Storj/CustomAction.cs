using System;
using System.Windows.Forms;
using Microsoft.Deployment.WindowsInstaller;

namespace Storj
{
    public class CustomActions
    {
        [CustomAction]
        public static ActionResult ValidateWallet(Session session)
        {
            string wallet = session["STORJ_WALLET"];

            if (string.IsNullOrEmpty(wallet))
            {
                return Error(session, "The wallet address cannot be empty.");
            }

            if (!wallet.StartsWith("0x"))
            {
                return Error(session, "The wallet address must start with a '0x' prefix.");
            }

            // Remove 0x prefix
            wallet = wallet.Substring(2);

            if (wallet.Length != 40)
            {
                return Error(session, "The wallet address must have 40 characters after the '0x' prefix.");
            }

            // TODO validate address checksum

            // Wallet is valid
            session["STORJ_WALLET_VALID"] = "1";
            return ActionResult.Success;
        }

        public static ActionResult Error(Session session, string msg)
        {
            MessageBox.Show(
                msg,
                "Invalid Wallet",
                MessageBoxButtons.OK,
                MessageBoxIcon.Error);

            session["STORJ_WALLET_VALID"] = String.Empty;
            return ActionResult.Success;
        }
    }
}
