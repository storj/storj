import argparse
import logging
import os
import subprocess
import sys
from enum import Enum

# Setting up basic logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')


class Section(Enum):
    GENERAL = "General"
    MULTINODE = "Multinode"
    SATELLITE = "Satellite"
    STORAGENODE = "Storagenode"
    TEST = "Test"
    UPLINK = "Uplink"


GITHUB_LINK = "[{0}](https://github.com/storj/storj/commit/{0})"


def git_ref_field(from_ref, to_ref):
    """
    Executes a git command to find the difference in commits between two references.
    Assumes 'from_ref' and 'to_ref' are valid Git references.

    Args:
        from_ref (str): The source reference.
        to_ref (str): The target reference.

    Returns:
        str: A string containing the git commit differences.
    """
    cmd = ["git", "cherry", from_ref, to_ref, "-v"]
    try:
        result = subprocess.run(cmd, text=True, capture_output=True, check=True)
        return result.stdout
    except subprocess.CalledProcessError as e:
        logging.error(f"Error executing git command: {e.stderr}")
        raise


def validate_git_refs(from_ref, to_ref):
    """
    Validates the provided Git references.

    Args:
        from_ref (str): The source reference.
        to_ref (str): The target reference.

    Returns:
        bool: True if references are valid, False otherwise.
    """
    for ref in [from_ref, to_ref]:
        result = subprocess.run(["git", "rev-parse", "--verify", ref], text=True, capture_output=True)
        if result.returncode != 0:
            logging.error(f"Invalid Git reference: {ref}")
            return False
    return True


def categorize_commit(commit, section_dict):
    """
    Categorizes a single commit into the appropriate section.
    Handles unexpected commit formats by logging a warning and defaulting to the GENERAL section.

    Args:
        commit (str): A git commit message.
        section_dict (dict): Dictionary of sections.

    Returns:
        None
    """
    try:
        commit_category = commit[42:].split(":")[0].lower()
        for category in section_dict:
            if category.name.lower() in commit_category:
                section_dict[category].append(generate_line(commit))
                return
        section_dict[Section.GENERAL].append(generate_line(commit))
    except IndexError:
        logging.warning(f"Unexpected commit format: {commit}")
        section_dict[Section.GENERAL].append(generate_line(commit))


def generate_changelog(commits):
    """
    Generates a formatted changelog from a string of commits.
    Args:
        commits (str): A string containing git commit messages.
    Returns:
        str: The formatted changelog.
    """
    if not commits:
        return "No new commits found or error occurred."

    changelog = "# Changelog\n"
    section_dict = {s: [] for s in Section}

    for commit in commits.splitlines():
        categorize_commit(commit, section_dict)

    for title, lines in section_dict.items():
        if lines:
            changelog += f'### {title.value}\n' + ''.join(lines)

    return changelog


def generate_line(commit):
    """
    Formats a single commit line for the changelog.
    Args:
        commit (str): A git commit message.
    Returns:
        str: The formatted commit line.
    """
    return f"- {GITHUB_LINK.format(commit[2:9])} {commit[42:]}\n"


def prompt_for_refs(args):
    """
    Prompts user for 'from_ref' and 'to_ref' if not provided.
    Args:
        args: Parsed command-line arguments.
    Returns:
        None
    """
    if not args.from_ref:
        args.from_ref = input("Enter the starting Git reference (from_ref): ")
    if not args.to_ref:
        args.to_ref = input("Enter the ending Git reference (to_ref): ")


def main():
    """
    Main function to parse arguments, validate them, and print the changelog.
    If run interactively, prompts the user for input.
    """
    parser = argparse.ArgumentParser(description="Generate a sorted changelog from Git commits.")
    parser.add_argument("from_ref", nargs='?', help="The ref to show the path from")
    parser.add_argument("to_ref", nargs='?', help="The ref to show the path to")

    args = parser.parse_args()

    # Check if the script is running interactively
    if os.isatty(sys.stdin.fileno()):
        prompt_for_refs(args)

    if not (args.from_ref and args.to_ref) or not validate_git_refs(args.from_ref, args.to_ref):
        parser.print_help()
        sys.exit(1)

    try:
        commits = git_ref_field(args.from_ref, args.to_ref)
        changelog = generate_changelog(commits)
        print(changelog)
    except Exception as e:
        logging.error(f"An error occurred: {str(e)}")

if __name__ == "__main__":
    main()
