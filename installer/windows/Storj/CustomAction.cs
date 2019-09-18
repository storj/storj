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

        [CustomAction]
        public static ActionResult ValidateStorage(Session session)
        {
            string storageStr = session["STORJ_STORAGE"];

            if (string.IsNullOrEmpty(storageStr))
            {
                session["STORJ_STORAGE_VALID"] = "The value cannot be empty.";
                return ActionResult.Success;
            }

            if (!double.TryParse(storageStr, out double storage))
            {
                session["STORJ_STORAGE_VALID"] = string.Format("'{0}' is not a valid number.", storageStr);
                return ActionResult.Success;
            }

            if (storage < 0.5) {
                session["STORJ_STORAGE_VALID"] = "The allocated disk space cannot be less than 0.5 TB.";
                return ActionResult.Success;
            }

            // Allocated Storage value is valid
            session["STORJ_STORAGE_VALID"] = "1";
            return ActionResult.Success;
        }

        [CustomAction]
        public static ActionResult ValidateBandwidth(Session session)
        {
            string bandwidthStr = session["STORJ_BANDWIDTH"];

            if (string.IsNullOrEmpty(bandwidthStr))
            {
                session["STORJ_BANDWIDTH_VALID"] = "The value cannot be empty.";
                return ActionResult.Success;
            }

            if (!double.TryParse(bandwidthStr, out double bandwidth))
            {
                session["STORJ_BANDWIDTH_VALID"] = string.Format("'{0}' is not a valid number.", bandwidthStr);
                return ActionResult.Success;
            }

            if (bandwidth < 2.0)
            {
                session["STORJ_BANDWIDTH_VALID"] = "The allocated bandwidth cannot be less than 2 TB.";
                return ActionResult.Success;
            }

            // Allocated Bandwidth value is valid
            session["STORJ_BANDWIDTH_VALID"] = "1";
            return ActionResult.Success;
        }
    }
}
