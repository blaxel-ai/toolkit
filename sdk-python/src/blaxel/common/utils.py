"""
This module provides utility functions for file operations within Blaxel.
It includes functions to copy folders and synchronize directory contents efficiently.
"""

import filecmp
import os
import shutil


def copy_folder(source_folder: str, destination_folder: str):
    """
    Copies the contents of the source folder to the destination folder.

    This function recursively copies all files and subdirectories from the `source_folder` to the `destination_folder`.
    It ensures that existing files are only overwritten if they differ from the source.

    Parameters:
        source_folder (str): The path to the source directory.
        destination_folder (str): The path to the destination directory.

    Raises:
        FileNotFoundError: If the source folder does not exist.
        PermissionError: If the program lacks permissions to read from the source or write to the destination.
    """
    for file in os.listdir(source_folder):
        if os.path.isdir(f"{source_folder}/{file}"):
            if not os.path.exists(f"{destination_folder}/{file}"):
                os.makedirs(f"{destination_folder}/{file}")
            copy_folder(f"{source_folder}/{file}", f"{destination_folder}/{file}")
        elif not os.path.exists(f"{destination_folder}/{file}") or not filecmp.cmp(
            f"{source_folder}/{file}", f"{destination_folder}/{file}"
        ):
            shutil.copy(f"{source_folder}/{file}", f"{destination_folder}/{file}")
