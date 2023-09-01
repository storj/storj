#!/usr/bin/env bash

# error codes
# 0 - exited without problems
# 1 - parameters not supported were used or unexpected error
# 2 - OS not supported by this script
# 3 - installed version is up to date
# 4 - supported unzip tools are not available

set -e

unzip_tools_list=('unzip' '7z' 'busybox')
os_arg=""
os_type_arg=""
download_only=false
download_path=""

usage() { echo "Usage: sudo -v ; curl https://raw.githubusercontent.com/storj/storj/main/scripts/install-uplink.sh | sudo bash [-s <os> -t <os-type> -o <output directory>]" 1>&2; exit 1; }

# Option parsing
while getopts "s:t:o::" opt; do
  case $opt in
    s)
      os_arg="$OPTARG"
      ;;
    t)
      os_type_arg="$OPTARG"
      ;;
    o)
      download_only=true
      if [ "$OPTARG" != "" ]; then
        download_path="$OPTARG"
      fi
			if [ ! -d "$download_path" ]; then
				mkdir -p "$download_path"
			fi
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
  esac
done

# create tmp directory and move to it with macOS compatibility fallback
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t 'uplink-install.XXXXXXXXXX')
cd "$tmp_dir"

# unzip tool check
set +e
for tool in ${unzip_tools_list[*]}; do
  trash=$(hash "$tool" 2>>errors)
  if [ "$?" -eq 0 ]; then
    unzip_tool="$tool"
    break
  fi
done
set -e

if [ -z "$unzip_tool" ]; then
  echo "Supported unzip tools not available. Install unzip, 7z, or busybox."
  exit 4
fi

# Check installed version
installed_version=$(uplink version 2>>errors | grep 'Version:' | awk '{print $2}' || echo "none")

# Fetch latest version from GitHub
latest_version=$(curl -s https://api.github.com/repos/storj/storj/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ "$installed_version" = "$latest_version" ] && [ "$download_only" != true ]; then
  echo "The latest version of uplink ${installed_version} is already installed."
  exit 3
fi

# OS detection
if [ -z "$os_arg" ]; then
  OS="$(uname)"
else
  OS="$os_arg"
fi

case $OS in
	Linux|linux) OS='linux' ;;
	FreeBSD|freebsd) OS='freebsd' ;;
	Darwin|darwin) OS='darwin' ;;
	*) echo 'OS not supported'; exit 2 ;;
esac

if [ -z "$os_type_arg" ]; then
  OS_type="$(uname -m)"
else
  OS_type="$os_type_arg"
fi

case "$OS_type" in
  x86_64|amd64) OS_type='amd64' ;;
  aarch64|arm64) OS_type='arm64' ;;
  *) echo 'OS type not supported'; exit 2 ;;
esac

# Download and unzip
download_link="https://github.com/storj/storj/releases/latest/download/uplink_${OS}_${OS_type}.zip"
uplink_zip="uplink_${OS}_${OS_type}.zip"

curl -L -OfsS "$download_link"

unzip_dir="tmp_unzip_dir_for_uplink"
echo $unzip_dir

case "$unzip_tool" in
  'unzip') unzip -a "$uplink_zip" -d "$unzip_dir" ;;
  '7z') 7z x "$uplink_zip" "-o$unzip_dir" ;;
  'busybox') mkdir -p "$unzip_dir"; busybox unzip "$uplink_zip" -d "$unzip_dir" ;;
esac


if [ "$download_only" = true ]; then
  cd -
  install -m 755 $tmp_dir/$unzip_dir/uplink $download_path/uplink
  echo "Downloaded and unzipped to $download_path"
  exit 0
fi

cd $unzip_dir

# Install uplink
case "$OS" in
  'linux'|'freebsd')
    install -m 755 -o root -g root uplink /usr/bin/uplink
    ;;
  'darwin')
    install -m 755 uplink /usr/local/bin/uplink
    ;;
  *) echo 'OS not supported'; exit 2 ;;
esac

version=$(uplink version 2>>errors | head -n 1)

echo "${version} has successfully installed."
echo 'Now run "uplink setup" for configuration.'
exit 0

