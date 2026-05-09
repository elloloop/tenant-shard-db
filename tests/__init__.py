"""Top-level tests namespace.

The actual Python tests live under ``tests/python/``. This file
exists so ``from tests.python._test_schemas import X`` resolves —
pytest's rootdir + pythonpath config doesn't reliably treat
``tests`` as an implicit namespace package on every host (CI failed
without this file even though local runs passed via cwd-based
import resolution). Sibling ``tests/go/`` and ``tests/contract/``
are language-foreign and don't need __init__.py.
"""
