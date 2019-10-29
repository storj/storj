using Microsoft.Deployment.WindowsInstaller;
using System;
using System.Globalization;
using System.IO;
using System.Text.RegularExpressions;

namespace Storj
{
    public class CustomActions
    {
        private const long GB = 1000 * 1000 * 1000;
        private const long TB = (long) 1000 * 1000 * 1000 * 1000;
        private const long MinFreeSpace = 550 * GB; // (500 GB + 10% overhead)

        [CustomAction]
        public static ActionResult ValidateIdentityDir(Session session)
        {
            string identityDir = session["IDENTITYDIR"];

            if (string.IsNullOrEmpty(identityDir))
            {
                session["STORJ_IDENTITYDIR_VALID"] = "You must select an identity folder.";
                return ActionResult.Success;
            }

            if (!Directory.Exists(identityDir))
            {
                session["STORJ_IDENTITYDIR_VALID"] = string.Format("Folder '{0}' does not exist.", identityDir);
                return ActionResult.Success;
            }

            if (!File.Exists(Path.Combine(identityDir, "ca.cert")))
            {
                session["STORJ_IDENTITYDIR_VALID"] = "File 'ca.cert' not found in the selected folder.";
                return ActionResult.Success;
            }

            if (!File.Exists(Path.Combine(identityDir, "ca.key")))
            {
                session["STORJ_IDENTITYDIR_VALID"] = "File 'ca.key' not found in the selected folder.";
                return ActionResult.Success;
            }

            if (!File.Exists(Path.Combine(identityDir, "identity.cert")))
            {
                session["STORJ_IDENTITYDIR_VALID"] = "File 'identity.cert' not found in the selected folder.";
                return ActionResult.Success;
            }

            if (!File.Exists(Path.Combine(identityDir, "identity.key")))
            {
                session["STORJ_IDENTITYDIR_VALID"] = "File 'identity.key' not found in the selected folder.";
                return ActionResult.Success;
            }

            // Identity dir is valid
            session["STORJ_IDENTITYDIR_VALID"] = "1";
            return ActionResult.Success;
        }

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
        public static ActionResult ValidateStorageDir(Session session)
        {
            string identityDir = session["STORAGEDIR"];

            if (string.IsNullOrEmpty(identityDir))
            {
                session["STORJ_STORAGEDIR_VALID"] = "You must select a storage folder.";
                return ActionResult.Success;
            }

            DirectoryInfo dir = new DirectoryInfo(identityDir);
            DriveInfo drive = new DriveInfo(dir.Root.FullName);

            // TODO: Find a way to calculate the available free space + total size of existing pieces
            if (drive.TotalSize < MinFreeSpace)
            {
                session["STORJ_STORAGEDIR_VALID"] = string.Format("The selected drive '{0}' has only {1:0.##} GB disk size. The minimum required is 550 GB.",
                    drive.Name, decimal.Divide(drive.TotalSize, GB));
                return ActionResult.Success;
            }

            // Storage dir is valid
            session["STORJ_STORAGEDIR_VALID"] = "1";
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

            if (!double.TryParse(storageStr, NumberStyles.Number, CultureInfo.CreateSpecificCulture("en-US"), out double storage))
            {
                session["STORJ_STORAGE_VALID"] = string.Format("'{0}' is not a valid number.", storageStr);
                return ActionResult.Success;
            }

            if (storage < 0.5) {
                session["STORJ_STORAGE_VALID"] = "The allocated disk space cannot be less than 0.5 TB.";
                return ActionResult.Success;
            }

            DirectoryInfo dir = new DirectoryInfo(session["STORAGEDIR"]);
            DriveInfo drive = new DriveInfo(dir.Root.FullName);
            long storagePlusOverhead = Convert.ToInt64(storage * 1.1 * TB);

            // TODO: Find a way to calculate the available free space + total size of existing pieces
            if (drive.TotalSize < storagePlusOverhead)
            {
                session["STORJ_STORAGE_VALID"] = string.Format("The disk size ({0:0.##} TB) on the selected drive {1} is less than the allocated disk space plus the 10% overhead ({2:0.##} TB total).",
                    decimal.Divide(drive.TotalSize, TB), drive.Name, decimal.Divide(storagePlusOverhead, TB));
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

            if (!double.TryParse(bandwidthStr, NumberStyles.Number, CultureInfo.CreateSpecificCulture("en-US"), out double bandwidth))
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

        [CustomAction]
        public static ActionResult ExtractInstallDir(Session session)
        {
            string line = session["STORJ_SERVICE_COMMAND"];
            session.Log($"ExtractInstallDir registry value: {line}");

            Regex pattern = new Regex(@"--config-dir ""(?<installDir>.*)""");
            Match match = pattern.Match(line);
            string path = match.Groups["installDir"].Value;
            session.Log($"ExtractInstallDir extracted path: {path}");

            session["STORJ_INSTALLDIR"] = path;
            return ActionResult.Success;
        }
    }
}
