#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()
path = root / "frontend/src/pages/inbounds/list/useInboundColumns.tsx"
text = path.read_text(encoding="utf-8")

old = '''          } else if (record.protocol === 'mieru') {
            const transport = String(supplementalSettings.transport || 'TCP').toUpperCase();
            tags.push(<Tag key="n" color="green">{transport === 'UDP' ? 'UDP' : 'TCP'}</Tag>);
          } else if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {'''
new = '''          } else if (record.protocol === 'mieru') {
            // Mieru can expose the primary binding plus extra bindings using a
            // different transport. Badges must reflect the saved configuration,
            // not a protocol-wide constant or only the primary binding.
            const transports = new Set<string>();
            const primaryTransport = String(supplementalSettings.transport || 'TCP').toUpperCase();
            transports.add(primaryTransport === 'UDP' ? 'UDP' : 'TCP');
            const additionalBindings = supplementalSettings.additionalPortBindings;
            if (Array.isArray(additionalBindings)) {
              for (const binding of additionalBindings) {
                if (!binding || typeof binding !== 'object') continue;
                const configured = String((binding as Record<string, unknown>).transport || 'TCP').toUpperCase();
                transports.add(configured === 'UDP' ? 'UDP' : 'TCP');
              }
            }
            if (transports.has('TCP')) tags.push(<Tag key="n-tcp" color="green">TCP</Tag>);
            if (transports.has('UDP')) tags.push(<Tag key="n-udp" color="green">UDP</Tag>);
          } else if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {'''

count = text.count(old)
if count != 1:
    raise SystemExit(f"v26 Mieru dynamic transport badge anchor: expected one, found {count}")
path.write_text(text.replace(old, new, 1), encoding="utf-8")
print("V26 Mieru configuration-driven transport badges applied.")
