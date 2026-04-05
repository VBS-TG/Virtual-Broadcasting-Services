#!/usr/bin/env python3
"""
Brave input.add_element() uses:

    if not input_bin.add(e):

PyGObject 綁定裡 Gst.Bin.add() 常回傳 None（void），`if not None` 為 True，
導致誤報「Unable to add element」並 return None，後續 intervideosink.set_property 崩潰。

改為先檢查元素是否建立成功，再呼叫 add(e)，與 mixers/mixer.add_element 一致。
"""
import sys
from pathlib import Path


OLD = """        e = Gst.ElementFactory.make(factory_name, name)
        if not input_bin.add(e):
            self.logger.error('Unable to add element %s' % factory_name)
            return None
        return e"""

NEW = """        e = Gst.ElementFactory.make(factory_name, name)
        if not e:
            self.logger.error('Unable to create element %s' % factory_name)
            return None
        input_bin.add(e)
        return e"""


def main() -> None:
    p = Path("brave/inputs/input.py")
    if not p.is_file():
        print("patch_brave_input_add_element: brave/inputs/input.py not found", file=sys.stderr)
        sys.exit(1)
    t = p.read_text(encoding="utf-8")
    if "if not input_bin.add(e):" not in t:
        if NEW.splitlines()[0] in t and "input_bin.add(e)" in t:
            print("patch_brave_input_add_element: already applied")
            return
        print("patch_brave_input_add_element: expected pattern not found; upstream may have changed", file=sys.stderr)
        sys.exit(1)
    if OLD not in t:
        print("patch_brave_input_add_element: add_element block mismatch; check Brave version", file=sys.stderr)
        sys.exit(1)
    p.write_text(t.replace(OLD, NEW, 1), encoding="utf-8")
    print("patch_brave_input_add_element: fixed Gst.Bin.add() truthiness for PyGObject")


if __name__ == "__main__":
    main()
