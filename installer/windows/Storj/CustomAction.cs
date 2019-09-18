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
                session["STORJ_WALLET_VALID"] = "The wallet address cannot be empty.";
                return ActionResult.Success;
            }

            if (!wallet.StartsWith("0x"))
            {
                session["STORJ_WALLET_VALID"] = "The wallet address must start with a '0x' prefix.";
                return ActionResult.Success;
            }

            // Remove 0x prefix
            wallet = wallet.Substring(2);

            if (wallet.Length != 40)
            {
                session["STORJ_WALLET_VALID"] = "The wallet address must have 40 characters after the '0x' prefix.";
                return ActionResult.Success;
            }

            // TODO validate address checksum

            // Wallet is valid
            session["STORJ_WALLET_VALID"] = "1";
            return ActionResult.Success;
        }
    }
}
