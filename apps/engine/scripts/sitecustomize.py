"""Compatibility shims for legacy libs on Python 3.10+.

Brave currently depends on sanic 19.x which still imports collection ABCs
from `collections` (removed in Python 3.10). Python auto-imports this module
at interpreter startup (if present on sys.path), so we patch aliases here.
"""

from collections import abc
import collections


_ALIASES = (
    "Mapping",
    "MutableMapping",
    "Sequence",
    "MutableSequence",
    "Set",
    "MutableSet",
)

for _name in _ALIASES:
    if not hasattr(collections, _name):
        setattr(collections, _name, getattr(abc, _name))
