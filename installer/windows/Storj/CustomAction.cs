using Microsoft.Deployment.WindowsInstaller;
using System;
using System.Globalization;
using System.IO;
using System.IO.Abstractions;
using System.Text.RegularExpressions;

namespace Storj
{
    public class CustomActions
    {

        [CustomAction]
        public static ActionResult ValidateIdentityDir(Session session)
        {
            string identityDir = session["IDENTITYDIR"];

            try
            {
                new CustomActionRunner().ValidateIdentityDir(identityDir);
            }
            catch (ArgumentException e)
            {
                // Identity dir is invalid
                session["STORJ_IDENTITYDIR_VALID"] = e.Message;
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

            try
            {
                new CustomActionRunner().ValidateWallet(wallet);
            } catch (ArgumentException e)
            {
                // Wallet is invalid
                session["STORJ_WALLET_VALID"] = e.Message;
                return ActionResult.Success;
            }

            // Wallet is valid
            session["STORJ_WALLET_VALID"] = "1";
            return ActionResult.Success;
        }

        [CustomAction]
        public static ActionResult ValidateStorageDir(Session session)
        {
            string storageDir = session["STORAGEDIR"];

            try
            {
                new CustomActionRunner().ValidateStorageDir(storageDir);
            }
            catch (ArgumentException e)
            {
                // Storage dir is invalid
                session["STORJ_STORAGEDIR_VALID"] = e.Message;
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
            string storageDir = session["STORAGEDIR"];

            try
            {
                new CustomActionRunner().ValidateStorage(storageStr, storageDir);
            }
            catch (ArgumentException e)
            {
                // Allocated Storage is invalid
                session["STORJ_STORAGE_VALID"] = e.Message;
                return ActionResult.Success;
            }

            // Allocated Storage value is valid
            session["STORJ_STORAGE_VALID"] = "1";
            return ActionResult.Success;
        }

        [CustomAction]
        public static ActionResult ExtractInstallDir(Session session)
        {
            string line = session["STORJ_SERVICE_COMMAND"];
            session.Log($"ExtractInstallDir registry value: {line}");

            string path = new CustomActionRunner().ExtractInstallDir(line);
            session.Log($"ExtractInstallDir extracted path: {path}");

            session["STORJ_INSTALLDIR"] = path;
            return ActionResult.Success;
        }
    }

    public class CustomActionRunner
    {
        public const long GB = 1000 * 1000 * 1000;
        public const long TB = (long)1000 * 1000 * 1000 * 1000;
        public const long MinFreeSpace = 550 * GB; // (500 GB + 10% overhead)

        private readonly IFileSystem fs;

        public CustomActionRunner() : this(fs: new FileSystem())
        { 
        }

        public CustomActionRunner(IFileSystem fs)
        {
            this.fs = fs;
        }

        public void ValidateIdentityDir(string identityDir)
        {
            if (string.IsNullOrEmpty(identityDir))
            {
                throw new ArgumentException("You must select an identity folder.");
            }

            if (!fs.Directory.Exists(identityDir))
            {
                throw new ArgumentException(string.Format("Folder '{0}' does not exist.", identityDir));
            }

            if (!fs.File.Exists(Path.Combine(identityDir, "ca.cert")))
            {
                throw new ArgumentException("File 'ca.cert' not found in the selected folder.");
            }

            if (!fs.File.Exists(Path.Combine(identityDir, "identity.cert")))
            {
                throw new ArgumentException("File 'identity.cert' not found in the selected folder.");
            }

            if (!fs.File.Exists(Path.Combine(identityDir, "identity.key")))
            {
                throw new ArgumentException("File 'identity.key' not found in the selected folder.");
            }
        }

        public void ValidateWallet(string wallet)
        {
            if (string.IsNullOrEmpty(wallet))
            {
                throw new ArgumentException("The payout address cannot be empty.");
            }

            if (!wallet.StartsWith("0x"))
            {
                throw new ArgumentException("The payout address must start with a '0x' prefix.");
            }

            // Remove 0x prefix
            wallet = wallet.Substring(2);

            if (wallet.Length != 40)
            {
                throw new ArgumentException("The payout address must have 40 characters after the '0x' prefix.");
            }

            // TODO validate address checksum
        }

        public void ValidateStorageDir(string storageDir)
        { 
            if (string.IsNullOrEmpty(storageDir))
            {
                throw new ArgumentException("You must select a storage folder.");
            }

            IDirectoryInfo dir = fs.DirectoryInfo.FromDirectoryName(storageDir);
            IDriveInfo drive = fs.DriveInfo.FromDriveName(dir.Root.FullName);

            // TODO: Find a way to calculate the available free space + total size of existing pieces
            if (drive.TotalSize < MinFreeSpace)
            {
                throw new ArgumentException(string.Format("The selected drive '{0}' has only {1:0.##} GB disk size. The minimum required is 550 GB.",
                    drive.Name, decimal.Divide(drive.TotalSize, GB)));
            }
        }

        public void ValidateStorage(string storageStr, string storageDir)
        {
            if (string.IsNullOrEmpty(storageStr))
            {
                throw new ArgumentException("The value cannot be empty.");
            }

            if (!double.TryParse(storageStr, NumberStyles.Number, CultureInfo.CreateSpecificCulture("en-US"), out double storage))
            {
                throw new ArgumentException(string.Format("'{0}' is not a valid number.", storageStr));
            }

            if (storage < 0.5)
            {
                throw new ArgumentException("The allocated disk space cannot be less than 0.5 TB.");
            }

            if (string.IsNullOrEmpty(storageDir))
            {
                throw new ArgumentException("The storage directory cannot be empty");
            }

            long storagePlusOverhead;
            try
            {
                storagePlusOverhead = Convert.ToInt64(storage * 1.1 * TB);
            }
            catch (OverflowException)
            {
                throw new ArgumentException(string.Format("{0} TB is too large value for allocated storage.", storage));
            }

            IDirectoryInfo dir = fs.DirectoryInfo.FromDirectoryName(storageDir);
            IDriveInfo drive = fs.DriveInfo.FromDriveName(dir.Root.FullName);

            // TODO: Find a way to calculate the available free space + total size of existing pieces
            if (drive.TotalSize < storagePlusOverhead)
            {
                throw new ArgumentException(string.Format("The disk size ({0:0.##} TB) on the selected drive {1} is less than the allocated disk space plus the 10% overhead ({2:0.##} TB total).",
                    decimal.Divide(drive.TotalSize, TB), drive.Name, decimal.Divide(storagePlusOverhead, TB)));
            }
        }

        public string ExtractInstallDir(string serviceCmd)
        {
            if (string.IsNullOrEmpty(serviceCmd))
            {
                return null;
            }

            Regex pattern = new Regex(@"--config-dir ""(?<installDir>.*)""");
            Match match = pattern.Match(serviceCmd);
            string installDir =  match.Groups["installDir"].Value;

            if (string.IsNullOrEmpty(installDir))
            {
                return null;
            }

            return installDir;
        }
    }


}
