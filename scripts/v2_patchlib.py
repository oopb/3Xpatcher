from pathlib import Path

class Patcher:
    def __init__(self, root):
        self.root = Path(root).resolve()

    def read(self, rel):
        return (self.root / rel).read_text()

    def write(self, rel, value):
        (self.root / rel).write_text(value)

    def rep(self, rel, old, new, count=1):
        value = self.read(rel)
        if new in value and old not in value:
            return
        if old not in value:
            raise SystemExit(f"{rel}: patch point not found:\n{old[:180]}")
        self.write(rel, value.replace(old, new, count))
