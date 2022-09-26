#!/usr/bin/env python
# Example of usage: changelog.py <old-release-tag> <new-release-tag>


import argparse
import subprocess

GENERAL = "General"
SATELLITE = "Satellite"
STORAGENODE = "Storagenode"
TEST = "Test"
UPLINK = "Uplink"
GITHUB_LINK = "[{0}](https://github.com/storj/storj/commit/{0})"


def git_ref_field(from_ref, to_ref):
    # Execute command to show diff without cherry-picks
    cmd = "git cherry {} {} -v | grep '^+'".format(from_ref, to_ref)
    ps = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    output = ps.communicate()[0]
    return output.decode()


def generate_changelog(commits):
    changelog = "# Changelog\n"
    section = {SATELLITE: [], STORAGENODE: [], TEST: [], UPLINK: [], GENERAL: []}

    # Sorting and populating the dictionary d with commit hash and message
    for commit in commits.splitlines():
        if TEST.lower() in commit[42:].split(":")[0]:
            section[TEST].append(generate_line(commit))
        elif STORAGENODE.lower() in commit[42:].split(":")[0]:
            section[STORAGENODE].append(generate_line(commit))
        elif UPLINK.lower() in commit[42:].split(":")[0]:
            section[UPLINK].append(generate_line(commit))
        elif SATELLITE.lower() in commit[42:].split(":")[0]:
            section[SATELLITE].append(generate_line(commit))
        else:
            section[GENERAL].append(generate_line(commit))

    for title in dict(sorted(section.items())):
        if section[title]:
            changelog += ('### {}\n'.format(title))
            for line in section[title]:
                changelog += line
    return changelog


def generate_line(commit):
    return "- {}{} \n".format(GITHUB_LINK.format(commit[2:9]), commit[42:])


def main():
    p = argparse.ArgumentParser(description=(
        "generate changelog sorted by topics."))
    p.add_argument("from_ref", help="the ref to show the path from")
    p.add_argument("to_ref", help="the ref to show the path to")
    args = p.parse_args()
    commits = git_ref_field(args.from_ref, args.to_ref)

    print(generate_changelog(commits))


if __name__ == "__main__":
    main()
