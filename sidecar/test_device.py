import os
import sys
import types
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from device import backend_name  # noqa: E402


def fake_mlx(core: types.ModuleType | None):
    """Install a fake mlx.core (None: mlx is not installed)."""
    for name in ("mlx", "mlx.core"):
        sys.modules.pop(name, None)
    if core is None:
        sys.modules["mlx"] = None  # import mlx raises ImportError
        return
    pkg = types.ModuleType("mlx")
    pkg.core = core
    sys.modules["mlx"] = pkg
    sys.modules["mlx.core"] = core


class BackendNameTest(unittest.TestCase):
    def tearDown(self):
        for name in ("mlx", "mlx.core"):
            sys.modules.pop(name, None)

    def test_the_gpu_is_named_when_mlx_names_it(self):
        core = types.ModuleType("mlx.core")
        core.device_info = lambda: {"device_name": "Apple M2 Pro", "memory_size": 1}
        fake_mlx(core)
        self.assertEqual(backend_name(), "Apple M2 Pro")

    def test_the_older_metal_namespace_is_read_too(self):
        core = types.ModuleType("mlx.core")
        core.metal = types.SimpleNamespace(device_info=lambda: {"device_name": "Apple M1"})
        fake_mlx(core)
        self.assertEqual(backend_name(), "Apple M1")

    def test_mlx_without_a_device_name_is_mlx(self):
        core = types.ModuleType("mlx.core")
        core.metal = types.SimpleNamespace(device_info=lambda: {"architecture": "applegpu_g13s"})
        fake_mlx(core)
        self.assertEqual(backend_name(), "mlx")

    def test_a_failing_query_is_mlx(self):
        core = types.ModuleType("mlx.core")

        def boom():
            raise RuntimeError("no Metal device")
        core.device_info = boom
        fake_mlx(core)
        self.assertEqual(backend_name(), "mlx")

    def test_without_mlx_it_is_mlx(self):
        fake_mlx(None)
        self.assertEqual(backend_name(), "mlx")


if __name__ == "__main__":
    unittest.main()
