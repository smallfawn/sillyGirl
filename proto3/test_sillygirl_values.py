import ast
import json
import unittest
from pathlib import Path


def load_value_functions():
    source = Path(__file__).with_name("sillygirl.py").read_text(encoding="utf-8")
    tree = ast.parse(source)
    selected = [
        node
        for node in tree.body
        if isinstance(node, ast.FunctionDef)
        and node.name in {"_serialize_bucket_value", "_transform_bucket_value"}
    ]
    namespace = {"json": json}
    exec(compile(ast.Module(body=selected, type_ignores=[]), "sillygirl.py", "exec"), namespace)
    return namespace["_serialize_bucket_value"], namespace["_transform_bucket_value"]


serialize_bucket_value, transform_bucket_value = load_value_functions()


class BucketValueEncodingTest(unittest.TestCase):
    def test_json_round_trip(self):
        value = {"items": [1, True, "ok"], "nested": {"count": 2}}
        encoded = serialize_bucket_value(value)
        self.assertTrue(encoded.startswith("o:"))
        self.assertEqual(transform_bucket_value(encoded), value)

    def test_rejects_non_json_value(self):
        with self.assertRaises(TypeError):
            serialize_bucket_value({1, 2})

    def test_rejects_legacy_pickle_value(self):
        with self.assertRaises(ValueError):
            transform_bucket_value("p:gASVCgAAAAAAAAB9lIwBeJRLAXMu")


if __name__ == "__main__":
    unittest.main()
