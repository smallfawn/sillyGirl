import ast
import io
import os
import re
import shutil
import stat
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path


def load_security_functions():
    source = Path(__file__).with_name("sillygirl.py").read_text(encoding="utf-8")
    tree = ast.parse(source)
    names = {
        "_compact_runtime_output",
        "_read_limited",
        "_safe_extract_tar",
        "_safe_extract_zip",
    }
    prefixes = {"_MAX_RELEASE_ARCHIVE_ENTRIES", "_MAX_RELEASE_UNPACKED_BYTES"}
    selected = []
    for node in tree.body:
        if isinstance(node, ast.FunctionDef) and node.name in names:
            selected.append(node)
        elif isinstance(node, ast.Assign) and any(
            isinstance(target, ast.Name) and target.id in prefixes for target in node.targets
        ):
            selected.append(node)
    namespace = {
        "os": os,
        "re": re,
        "shutil": shutil,
        "stat": stat,
        "tarfile": tarfile,
        "zipfile": zipfile,
    }
    exec(compile(ast.Module(body=selected, type_ignores=[]), "sillygirl.py", "exec"), namespace)
    return namespace


security = load_security_functions()


class RuntimeSecurityTest(unittest.TestCase):
    def test_read_limited(self):
        self.assertEqual(security["_read_limited"](io.BytesIO(b"1234"), 4), b"1234")
        with self.assertRaises(RuntimeError):
            security["_read_limited"](io.BytesIO(b"12345"), 4)

    def test_compact_runtime_output_bounds_error_text(self):
        compact = security["_compact_runtime_output"]("first\n\nsecond", 20)
        self.assertEqual(compact, "first second")
        self.assertEqual(security["_compact_runtime_output"]("123456", 5), "...56")

    def test_zip_rejects_traversal_and_size_limit(self):
        with tempfile.TemporaryDirectory() as root:
            archive = os.path.join(root, "payload.zip")
            with zipfile.ZipFile(archive, "w") as package:
                package.writestr("../outside.txt", b"bad")
            with self.assertRaises(RuntimeError):
                security["_safe_extract_zip"](archive, os.path.join(root, "out"))

            with zipfile.ZipFile(archive, "w") as package:
                package.writestr("inside.txt", b"1234")
            previous = security["_MAX_RELEASE_UNPACKED_BYTES"]
            security["_MAX_RELEASE_UNPACKED_BYTES"] = 3
            try:
                with self.assertRaises(RuntimeError):
                    security["_safe_extract_zip"](archive, os.path.join(root, "out"))
            finally:
                security["_MAX_RELEASE_UNPACKED_BYTES"] = previous

    def test_tar_rejects_symbolic_link(self):
        with tempfile.TemporaryDirectory() as root:
            archive = os.path.join(root, "payload.tar")
            with tarfile.open(archive, "w") as package:
                link = tarfile.TarInfo("link")
                link.type = tarfile.SYMTYPE
                link.linkname = "../outside.txt"
                package.addfile(link)
            with self.assertRaises(RuntimeError):
                security["_safe_extract_tar"](archive, os.path.join(root, "out"))

    def test_safe_archives_extract_regular_files(self):
        with tempfile.TemporaryDirectory() as root:
            zip_archive = os.path.join(root, "payload.zip")
            with zipfile.ZipFile(zip_archive, "w") as package:
                package.writestr("nested/value.txt", b"zip-ok")
            zip_target = os.path.join(root, "zip-out")
            security["_safe_extract_zip"](zip_archive, zip_target)
            self.assertEqual(Path(zip_target, "nested", "value.txt").read_bytes(), b"zip-ok")

            tar_archive = os.path.join(root, "payload.tar")
            payload = b"tar-ok"
            with tarfile.open(tar_archive, "w") as package:
                entry = tarfile.TarInfo("nested/value.txt")
                entry.size = len(payload)
                package.addfile(entry, io.BytesIO(payload))
            tar_target = os.path.join(root, "tar-out")
            security["_safe_extract_tar"](tar_archive, tar_target)
            self.assertEqual(Path(tar_target, "nested", "value.txt").read_bytes(), payload)


if __name__ == "__main__":
    unittest.main()
